package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const BaseURL = "http://api.crunchbase.com/v/1/"

var apiKey = flag.String("key", "", "CrunchBase API key")
var remoteMode = flag.Bool("remote", false, "Fetch from CrunchBase API instead of local filesystem")
var dataPath = flag.String("path", "./data", "Path to local data on the filesystem")
var concurrency = flag.Int("workers", 40, "Number of workers to fetch with")
var upload = flag.Bool("upload", false, "Upload the generated site to Rackspace")

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

var prefixPattern = regexp.MustCompile(`\b[a-z0-9]`)

func wordPrefixes(s string) map[string]bool {
	p := prefixPattern.FindAllString(strings.ToLower(s), -1)
	prefixes := make(map[string]bool)
	for _, prefix := range p {
		prefixes[prefix] = true
	}
	return prefixes
}

func getVC(permalink string) {
	res, err := Get("financial-organization/" + permalink)
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

	vc.RoundsByCode = make(map[string]int64)
	vc.RoundsByYear = make(map[int]int64)
	vc.RoundsByCompany = make(map[string]int64)
	vc.CompaniesByYear = make(map[int]int64)
	vc.RoundShares = make(IntSlice, 0, len(vc.Investments))
	vc.RoundSizes = make(IntSlice, 0, len(vc.Investments))
	vc.Partners = make(map[*VC]*Partner)
	vc.PartnersByRound = make(map[string][]int64)

	companiesByYear := make(map[int]map[string]bool)

	for _, inv := range vc.Investments {
		r := inv.Round
		cp := r.Company.Permalink

		var y int
		if r.Year != nil {
			y = *r.Year
		}
		rid := cp + ":" + strconv.Itoa(y) + ":" + r.Code

		vc.RoundsByCode[r.Code] += 1

		if r.Year != nil {
			year := *r.Year
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

		if inv.Round.Amount != nil && *inv.Round.Amount >= 1 {
			vc.RoundSizes = append(vc.RoundSizes, int64(*inv.Round.Amount))
		}

		IndexMutex.Lock()
		RoundVCs[rid] = append(RoundVCs[rid], vc)
		Rounds[rid] = *r
		IndexMutex.Unlock()
	}

	vc.RoundSizes.Sort()

	for year, companies := range companiesByYear {
		if year > 2004 {
			vc.CompaniesByYear[year] = int64(len(companies))
		}
	}
	vc.TotalCompanies = len(vc.RoundsByCompany)

	vc.YearRoundSet = make(IntSlice, 0, len(vc.RoundsByYear))
	for year, x := range vc.RoundsByYear {
		if year > 2004 {
			vc.YearRoundSet = append(vc.YearRoundSet, int64(x))
		}
	}
	vc.YearRoundSet.Sort()

	vc.YearCompanySet = make(IntSlice, 0, len(vc.CompaniesByYear))
	for _, x := range vc.CompaniesByYear {
		vc.YearCompanySet = append(vc.YearCompanySet, int64(x))
	}
	vc.YearCompanySet.Sort()

	vc.SeriesDist.Buckets = make([]BucketedInt, 0, len(vc.RoundsByCode))
	for _, b := range RoundCodeBuckets {
		if c, ok := vc.RoundsByCode[strings.ToLower(b)]; ok {
			if c > vc.SeriesDist.Max {
				vc.SeriesDist.Max = c
			}
			vc.SeriesDist.Buckets = append(vc.SeriesDist.Buckets, BucketedInt{b, c})
		}
	}

	cs := make([]int64, 0, len(vc.RoundsByCompany))
	for _, i := range vc.RoundsByCompany {
		cs = append(cs, int64(i))
	}
	vc.RoundCountDist = RoundCountBuckets.Aggregate(cs)
	vc.RaiseDist = RoundSizeBuckets.Aggregate(vc.RoundSizes)

	IndexMutex.Lock()
	VCs[vc.Permalink] = vc
	vcDataList = append(vcDataList, []string{vc.Permalink, vc.Name})
	for prefix, _ := range wordPrefixes(vc.Name) {
		vcNamePrefixes[prefix] = append(vcNamePrefixes[prefix], len(vcDataList)-1)
	}
	IndexMutex.Unlock()
}

func calculateVCs() {
	IndexMutex.RLock()
	defer IndexMutex.RUnlock()

	for rid, vcs := range RoundVCs {
		r := Rounds[rid]

		agg := func(vc *VC) {
			for _, v := range vcs {
				if v.Permalink == vc.Permalink {
					continue
				}

				if r.Year != nil {
					var p *Partner
					var ok bool
					if p, ok = vc.Partners[v]; !ok {
						p = &Partner{VC: v}
						vc.Partners[v] = p
					}
					vc.Partners[v].Rounds += 1

					year := *r.Year
					if p.FirstYear == 0 || p.FirstYear > year {
						p.FirstYear = year
					}
					if p.LastYear == 0 || p.LastYear < year {
						p.LastYear = year
					}
				}
			}

			if r.Amount != nil && *r.Amount >= 1 {
				vc.RoundShares = append(vc.RoundShares, RoundInt(*r.Amount/float64(len(vcs))))
			}

			vc.PartnerCountSet = append(vc.PartnerCountSet, int64(len(vcs)))

			if _, ok := vc.PartnersByRound[r.Code]; !ok {
				vc.PartnersByRound[r.Code] = make([]int64, 0, 1)
			}
			vc.PartnersByRound[r.Code] = append(vc.PartnersByRound[r.Code], int64(len(vcs))-1)
		}

		for _, vc := range vcs {
			agg(vc)
		}
	}

	for _, vc := range VCs {
		vc.RoundShares.Sort()
		vc.ShareDist = RoundShareBuckets.Aggregate(vc.RoundShares)

		vc.PartnerList = make(PartnerList, 0, len(vc.Partners))
		for _, p := range vc.Partners {
			if p.Rounds < 2 {
				continue
			}

			var r int64
			for y := p.FirstYear; y <= p.LastYear; y++ {
				r += p.VC.RoundsByYear[y]
			}
			p.Percentage = int64(math.Floor((float64(p.Rounds) / float64(r)) * 100))
			vc.PartnerList = append(vc.PartnerList, p)
		}
		sort.Sort(vc.PartnerList)

		vc.InvestorRoundDist.Buckets = make([]BucketedInt, 0, len(vc.PartnersByRound))
		for _, b := range RoundCodeBuckets {
			if cs, ok := vc.PartnersByRound[strings.ToLower(b)]; ok {
				c := RoundInt(Mean(cs))
				if c > vc.InvestorRoundDist.Max {
					vc.InvestorRoundDist.Max = c
				}
				vc.InvestorRoundDist.Buckets = append(vc.InvestorRoundDist.Buckets, BucketedInt{b, c})
			}
		}
	}

	for _, l := range vcNamePrefixes {
		sort.Sort(l)
	}
}

type VC struct {
	ID        int
	Name      string         `json:"name"`
	Permalink string         `json:"permalink"`
	URL       *string        `json:"homepage_url"`
	Overview  *template.HTML `json:"overview"`

	RoundsByCode    map[string]int64
	RoundsByYear    map[int]int64
	RoundsByCompany map[string]int64
	CompaniesByYear map[int]int64
	Partners        map[*VC]*Partner

	RoundShares    IntSlice
	RoundSizes     IntSlice
	YearCompanySet IntSlice
	YearRoundSet   IntSlice

	PartnerCountSet []int64
	PartnersByRound map[string][]int64

	SeriesDist        BucketedInts
	RoundCountDist    BucketedInts
	RaiseDist         BucketedInts
	ShareDist         BucketedInts
	InvestorRoundDist BucketedInts

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
	Percentage int64
	FirstYear  int
	LastYear   int

	VC *VC
}

type PartnerList []*Partner

func (p PartnerList) Len() int           { return len(p) }
func (p PartnerList) Less(i, j int) bool { return p[i].Rounds > p[j].Rounds }
func (p PartnerList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type BucketedInts struct {
	Max     int64
	Buckets []BucketedInt
}

type BucketedInt struct {
	Name  string
	Count int64
}

type WeightedIDs []int

func (w WeightedIDs) Len() int           { return len(w) }
func (w WeightedIDs) Less(i, j int) bool { return len(VCs[vcDataList[w[i]][0]].Investments) > len(VCs[vcDataList[w[j]][0]].Investments) }
func (w WeightedIDs) Swap(i, j int)      { w[i], w[j] = w[j], w[i] }

var (
	RoundCodeBuckets  = []string{"Angel", "Seed", "A", "B", "C", "D", "E", "F", "G"}
	RoundSizeBuckets  = Buckets("<500k", "500 - 999k", "1 - 1.49m", "1.5 - 2.9m", "3 - 6.9m", "7 - 14.9m", "15 - 29.9m", ">30m")
	RoundShareBuckets = Buckets("<50k", "50 - 149k", "150 - 499k", "500k - 1.9m", "2 - 4.9m", "5 - 9.9m", ">10m")
	RoundCountBuckets = Buckets("1", "2", "3", "4", "5", "6")
)

var (
	IndexMutex     = new(sync.RWMutex)
	VCs            = make(map[string]*VC)
	RoundVCs       = make(map[string][]*VC)
	Rounds         = make(map[string]Round)
	vcNamePrefixes = make(map[string]WeightedIDs)
	vcDataList     = [][]string{}
)

func apiURL(path string) string {
	return BaseURL + path + "?api_key=" + *apiKey
}

func MaybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

var doneCount int32 = 0
var total int

func Get(path string) (io.ReadCloser, error) {
	if *remoteMode {
		res, err := http.Get(apiURL(path + ".js"))
		if err != nil {
			return nil, err
		}
		if res.StatusCode == 504 { // retry once
			res, err = http.Get(apiURL(path + ".js"))
			if err != nil {
				return nil, err
			}
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("get %s - incorrect response code received - %d", path, res.StatusCode)
		}
		atomic.AddInt32(&doneCount, 1)
		fmt.Printf("\r%d/%d", doneCount, total)
		return res.Body, nil
	}

	return os.Open(*dataPath + "/" + path + ".json")
}

func fetcher(queue chan string, done chan bool) {
	for permalink := range queue {
		getVC(permalink)
	}
	done <- true
}

func Put(path string, r io.Reader) error {
	atomic.AddInt32(&doneCount, 1)
	fmt.Printf("\r%d/%d", doneCount, total)

	if *upload {
		return PutCloudFile(path, r)
	}

	f, err := os.Create("output/" + path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	return nil
}

func render(t *template.Template, vc *VC) error {
	r, w := io.Pipe()
	go func() {
		err := t.ExecuteTemplate(w, "vc.html", vc)
		if err != nil {
			fmt.Printf("%s: %s\n", vc.Permalink, err)
		}
		w.Close()
	}()

	return Put("firms/"+vc.Permalink+".html", r)
}

func renderIndexPage(t *template.Template) error {
	r, w := io.Pipe()
	go func() {
		err := t.ExecuteTemplate(w, "index.html", VCs)
		if err != nil {
			fmt.Printf("index.html: %s\n", err)
		}
		w.Close()
	}()

	return Put("index.html", r)
}

func renderIndexJSON() error {
	r, w := io.Pipe()
	go func() {
		err := json.NewEncoder(w).Encode(map[string]interface{}{"a": vcDataList, "b": vcNamePrefixes})
		if err != nil {
			fmt.Println("index.json:", err)
		}
		w.Close()
	}()

	return Put("index.json", r)
}

func renderer(t *template.Template, queue chan *VC, done chan bool) {
	for vc := range queue {
		err := render(t, vc)
		if err != nil {
			fmt.Printf("render error: %s\n", err)
		}
	}
	done <- true
}

func waitDone(done chan bool) {
	for i := 0; i < *concurrency; i++ {
		<-done
	}
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	done := make(chan bool, *concurrency)
	queue := make(chan string)
	for i := 0; i < *concurrency; i++ {
		go fetcher(queue, done)
	}

	list := getVCList()
	total = len(list)
	for _, vc := range list {
		queue <- vc.Link
	}
	close(queue)
	waitDone(done)

	calculateVCs()

	t := template.Must(template.New("vc").Funcs(template.FuncMap{
		"first":  First,
		"last":   Last,
		"mean":   Mean,
		"median": Median,
		"sum":    Sum,
		"round":  Roundf,
		"pround": PrettyRound,
		"itof":   Itof,
		"barh":   BarHeight,
		"barml":  BarMarginLabel,
		"barmp":  BarMarginPadding,
		"barmh":  BarMarginHeight,
		"asset":  AssetPath,
	}).ParseFiles("templates/vc.html", "templates/index.html"))

	writeAssets()

	doneCount = 0

	rqueue := make(chan *VC)
	for i := 0; i < *concurrency; i++ {
		go renderer(t, rqueue, done)
	}

	for _, vc := range VCs {
		rqueue <- vc
	}
	close(rqueue)
	waitDone(done)

	renderIndexPage(t)
	renderIndexJSON()
}
