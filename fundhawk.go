package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"runtime"
	"sort"
	"sync"
)

const BaseURL = "http://api.crunchbase.com/v/1/"

var apiKey = flag.String("key", "", "CrunchBase API key")
var remoteMode = flag.Bool("remote", false, "Fetch from CrunchBase API instead of local filesystem")
var dataPath = flag.String("path", "./data", "Path to local data on the filesystem")
var concurrency = flag.Int("workers", 50, "Number of workers to fetch with")

type Permalink struct {
	Link string `json:"permalink"`
}

type Permalinks []Permalink

func getList(category string) Permalinks {
	res, err := Get(category)
	MaybePanic(err)
	defer res.Close()

	l := make(Permalinks, 0)
	list := &l

	err = json.NewDecoder(res).Decode(list)
	MaybePanic(err)

	return *list
}

func getVCList() Permalinks {
	return getList("financial-organizations")
}

func getVC(permalink string) {
	res, err := Get("financial-organizations/" + permalink)
	if err != nil {
		fmt.Printf("getVC fetch error: %s - %s\n", permalink, err)
		return
	}
	defer res.Close()

	vc := new(VC)
	err = json.NewDecoder(res).Decode(vc)
	if err != nil {
		fmt.Printf("getVC parse error: %s - %s\n", permalink, err)
		return
	}

	if len(vc.Investments) == 0 {
		return
	}

	vc.RoundsByCode = make(map[string]int)
	vc.RoundsByYear = make(map[int]int)
	vc.RoundsByCompany = make(map[string]int)
	vc.CompaniesByYear = make(map[int]int)
	vc.RoundShares = make(sort.IntSlice, 0, len(vc.Investments))
	vc.RoundSizes = make(sort.IntSlice, len(vc.Investments))
	vc.Partners = make(map[*VC]*Partner)

	companiesByYear := make(map[int]map[string]bool)

	for i, inv := range vc.Investments {
		r := inv.Round
		cp := r.Company.Permalink

		if _, ok := vc.RoundsByCode[r.Code]; !ok {
			vc.RoundsByCode[r.Code] = 0
		}
		vc.RoundsByCode[r.Code] += 1

		if r.Year != nil {
			year := *r.Year
			if _, ok := vc.RoundsByYear[year]; !ok {
				vc.RoundsByYear[year] = 0
			}
			vc.RoundsByYear[year] += 1

			if _, ok := companiesByYear[year]; !ok {
				companiesByYear[year] = make(map[string]bool)
			}
			companiesByYear[year][cp] = true
		}

		if _, ok := vc.RoundsByCompany[cp]; !ok {
			vc.RoundsByCompany[cp] = 0
		}
		vc.RoundsByCompany[cp] += 1

		if inv.Round.Amount != nil {
			vc.RoundSizes[i] = int(*inv.Round.Amount)
		}

		IndexMutex.Lock()
		RoundVCs[*r] = append(RoundVCs[*r], vc)
		IndexMutex.Unlock()
	}

	vc.RoundSizes.Sort()

	for year, companies := range companiesByYear {
		vc.CompaniesByYear[year] = len(companies)
	}
	vc.TotalCompanies = len(vc.RoundsByCompany)

	vc.YearRoundSet = make(sort.IntSlice, 0, len(vc.RoundsByYear))
	for _, x := range vc.RoundsByYear {
		vc.YearRoundSet = append(vc.YearRoundSet, x)
	}
	vc.YearRoundSet.Sort()

	vc.YearCompanySet = make(sort.IntSlice, 0, len(vc.RoundsByCompany))
	for _, x := range vc.RoundsByCompany {
		vc.YearCompanySet = append(vc.YearCompanySet, x)
	}
	vc.YearCompanySet.Sort()

	IndexMutex.Lock()
	VCs[vc.Permalink] = vc
	IndexMutex.Unlock()
}

func calculateVCs() {
	IndexMutex.RLock()
	defer IndexMutex.RUnlock()

	for r, vcs := range RoundVCs {
		agg := func(vc *VC) {
			for _, v := range vcs {
				if v.Permalink == vc.Permalink {
					continue
				}

				var p *Partner
				if p, ok := vc.Partners[v]; !ok {
					p = &Partner{VC: v}
					vc.Partners[v] = p
				}
				vc.Partners[v].Rounds += 1

				if r.Year != nil {
					year := *r.Year
					if p.FirstYear == 0 || p.FirstYear > year {
						p.FirstYear = year
					}
					if p.LastYear == 0 || p.LastYear < year {
						p.LastYear = year
					}
				}
			}

			if r.Amount != nil {
				vc.RoundShares = append(vc.RoundShares, int(math.Floor(*r.Amount/float64(len(vcs))+0.5)))
			}
		}

		for _, vc := range vcs {
			agg(vc)
		}
	}

	for _, vc := range VCs {
		vc.RoundShares.Sort()

		vc.PartnerList = make(PartnerList, 0, len(vc.Partners))
		for _, p := range vc.Partners {
			r := 0
			for y := p.FirstYear; y <= p.LastYear; y++ {
				r += p.VC.RoundsByYear[y]
			}
			p.Percentage = int(math.Floor(((float64(r) / 100) * float64(p.Rounds)) + 0.5))
			vc.PartnerList = append(vc.PartnerList, p)
		}
		sort.Sort(vc.PartnerList)
	}
}

type VC struct {
	Name      string  `json:"name"`
	Permalink string  `json:"permalink"`
	URL       *string `json:"homepage_url"`

	RoundsByCode    map[string]int
	RoundsByYear    map[int]int
	RoundsByCompany map[string]int
	CompaniesByYear map[int]int
	Partners        map[*VC]*Partner

	RoundShares    sort.IntSlice
	RoundSizes     sort.IntSlice
	YearCompanySet sort.IntSlice
	YearRoundSet   sort.IntSlice

	PartnerList PartnerList

	TotalCompanies int

	Investments []struct {
		Round *Round `json:"funding_round"`
	} `json:"investments"`
}

type Round struct {
	Code    string   `json:"round_code"`
	Amount  *float64 `json:"raised_amount"`
	Year    *int     `json:"funded_year"`
	Company Company  `json:"company"`
}

type Company struct {
	Name      string `json:"name"`
	Permalink string `json:"permalink"`
}

type Partner struct {
	Rounds     int
	Percentage int
	FirstYear  int
	LastYear   int

	VC *VC
}

type PartnerList []*Partner

func (p PartnerList) Len() int           { return len(p) }
func (p PartnerList) Less(i, j int) bool { return p[i].Rounds < p[j].Rounds }
func (p PartnerList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

var (
	IndexMutex = new(sync.RWMutex)
	VCs        = make(map[string]*VC)
	RoundVCs   = make(map[Round][]*VC)
)

func apiURL(path string) string {
	return BaseURL + path + "?api_key=" + *apiKey
}

func Mean(p sort.IntSlice) (a float64) {
	if len(p) == 0 {
		return 0
	}
	for _, x := range p {
		a += float64(x)
	}
	return a / float64(len(p))
}

func Median(p sort.IntSlice) float64 {
	if len(p) == 0 {
		return 0
	}
	if len(p)%2 != 0 {
		return float64(p[len(p)/2])
	}
	i := len(p) / 2
	return float64(p[i]+p[i+1]) / 2
}

func First(p sort.IntSlice) int {
	if len(p) == 0 {
		return 0
	}
	return p[0]
}

func Last(p sort.IntSlice) int {
	if len(p) == 0 {
		return 0
	}
	return p[len(p)-1]
}

func MaybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

func Get(path string) (io.ReadCloser, error) {
	if *remoteMode {
		res, err := http.Get(apiURL(path + ".js"))
		if err != nil {
			return nil, err
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("get %s - incorrect response code received - %d", path, res.StatusCode)
		}
		return res.Body, nil
	}

	return os.Open(*dataPath + "/" + path + ".json")
}

func fetcher(queue chan string) {
	for permalink := range queue {
		getVC(permalink)
	}
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	queue := make(chan string)
	for i := 0; i < *concurrency; i++ {
		go fetcher(queue)
	}

	list := getVCList()
	for _, vc := range list {
		queue <- vc.Link
	}
	close(queue)
	calculateVCs()
}
