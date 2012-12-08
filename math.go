package main

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

func Sum(p []int64) (a int64) {
	for _, x := range p {
		a += x
	}
	return a
}

func Mean(p []int64) (a float64) {
	if len(p) == 0 {
		return 0
	}
	return float64(Sum(p)) / float64(len(p))
}

func Roundf(f float64) string {
	return strconv.FormatFloat(RoundFloat(f, 2), 'f', -1, 64)
}

func Median(p []int64) float64 {
	if len(p) == 0 {
		return 0
	}
	if len(p)%2 != 0 {
		return float64(p[len(p)/2])
	}
	i := len(p) / 2
	return float64(p[i]+p[i-1]) / 2
}

func First(p []int64) int64 {
	if len(p) == 0 {
		return 0
	}
	return p[0]
}

func Last(p []int64) int64 {
	if len(p) == 0 {
		return 0
	}
	return p[len(p)-1]
}

func PrettyRound(i float64) string {
	var suffix string
	var x float64

	if i >= 1000000000 {
		suffix = "B"
		x = i / 100000000
	} else if i >= 1000000 {
		suffix = "M"
		x = i / 100000
	} else if i >= 1000 {
		suffix = "K"
		x = i / 100
	}

	return strconv.FormatFloat(float64(RoundInt(x))/10, 'f', -1, 64) + suffix
}

func Itof(i int64) float64 {
	return float64(i)
}

// NiceNum finds a "nice" number approximately equal to x. Round the number if round is true, otherwise take the
// ceiling.  Described on Graphics Gems pg. 63
func NiceNum(x float64, round bool) float64 {
	exp := int(math.Floor(math.Log10(x)))
	f := x / math.Pow10(exp)

	var nf int
	if round {
		if f < 1.5 {
			nf = 1
		} else if f < 3 {
			nf = 2
		} else if f < 7 {
			nf = 5
		} else {
			nf = 10
		}
	} else {
		if f <= 1 {
			nf = 1
		} else if f <= 2 {
			nf = 2
		} else if f <= 5 {
			nf = 5
		} else {
			nf = 10
		}
	}

	return float64(nf) * math.Pow10(exp)
}

func RoundFloat(x float64, prec int) float64 {
	var rounder float64
	pow := math.Pow(10, float64(prec))
	intermed := x * pow
	_, frac := math.Modf(intermed)
	x = .5
	if frac < 0.0 {
		x = -.5
	}
	if frac >= x {
		rounder = math.Ceil(intermed)
	} else {
		rounder = math.Floor(intermed)
	}

	return rounder / pow
}

func RoundInt(n float64) int64 {
	return int64(RoundFloat(n, 0))
}

func BarHeight(max int64, n int64) int {
	return int(RoundInt((100 / float64(max)) * float64(n)))
}

const labelHeight = 20

func BarMarginPadding(n int) int {
	if n > 99 {
		return 0
	}
	if n < 20 {
		return 100 - n - labelHeight
	}
	return 100 - n
}

func BarMarginHeight(n int) int {
	if n < 20 {
		return labelHeight
	}
	return 0
}

func BarMarginLabel(n int) bool {
	if n >= 20 {
		return true
	}
	return false
}

type ValueBucket struct {
	Name string
	Min  int64
}

type ValueBuckets []ValueBucket

func (buckets ValueBuckets) Aggregate(values []int64) BucketedInts {
	agg := make(map[string]int64)

	incr := func(s string) {
		if _, ok := agg[s]; !ok {
			agg[s] = 0
		}
		agg[s] += 1
	}

	for _, x := range values {
		for i, b := range buckets {
			if x >= b.Min {
				if i < len(buckets)-1 && x >= buckets[i+1].Min {
					continue
				}
				incr(b.Name)
			}
		}
	}

	res := BucketedInts{Buckets: make([]BucketedInt, 0, len(agg))}
	for _, b := range buckets {
		if c, ok := agg[b.Name]; ok {
			if c > res.Max {
				res.Max = c
			}
			res.Buckets = append(res.Buckets, BucketedInt{b.Name, c})
		}
	}

	return res
}

func Buckets(names ...string) ValueBuckets {
	b := make([]ValueBucket, len(names))

	for i, n := range names {
		var x, min float64 = 1, 0

		j := strings.Index(n, " - ")
		if j != -1 && n[j-1] == 'k' {
			j -= 1
			x = 1000
		} else if j != -1 && n[j-1] == 'm' {
			j -= 1
			x = 1000000
		} else if n[len(n)-1] == 'k' {
			n = n[:len(n)-1]
			x = 1000
		} else if n[len(n)-1] == 'm' {
			n = n[:len(n)-1]
			x = 1000000
		}

		if j != -1 {
			n = n[:j]
		}

		if n[0] == '<' {
			b[i] = ValueBucket{names[i], 0}
			continue
		} else if n[0] == '>' {
			min, _ = strconv.ParseFloat(n[1:], 64)
			b[i] = ValueBucket{names[i], int64(min * x)}
			continue
		}

		min, _ = strconv.ParseFloat(n, 64)
		b[i] = ValueBucket{names[i], int64(min * x)}
	}

	return b
}

type IntSlice []int64

func (p IntSlice) Len() int           { return len(p) }
func (p IntSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p IntSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p IntSlice) Sort()              { sort.Sort(p) }
