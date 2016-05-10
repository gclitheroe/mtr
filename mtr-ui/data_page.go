package main

import (
	"bytes"
	"github.com/GeoNet/mtr/mtrpb"
	"github.com/golang/protobuf/proto"
	"net/http"
	"sort"
)

type dataPage struct {
	page
	Path    string
	Summary map[string]int
	Metrics []idCount
	Sites   []site
}

type sites []site

func (m sites) Len() int {
	return len(m)
}

func (m sites) Less(i, j int) bool {
	return m[i].SiteId < m[j].SiteId
}

func (m sites) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

type site struct {
	SiteId    string
	TypeCount int
	Types     []idCount
}

func dataPageHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {

	var err error

	if res := checkQuery(r, []string{}, []string{}); !res.ok {
		return res
	}

	// We create a page struct with variables to substitute into the loaded template
	p := dataPage{}
	p.Border.Title = "GeoNet MTR"

	if err = p.populateTags(); err != nil {
		return internalServerError(err)
	}

	if p.Summary, err = getDataSummary(); err != nil {
		return internalServerError(err)
	}

	p.Path = r.URL.Path
	if err = dataTemplate.ExecuteTemplate(b, "border", p); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func dataMetricsPageHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {

	var err error

	if res := checkQuery(r, []string{}, []string{}); !res.ok {
		return res
	}

	// We create a page struct with variables to substitute into the loaded template
	p := dataPage{}
	p.Path = r.URL.Path
	p.Border.Title = "GeoNet MTR"

	if err = p.populateTags(); err != nil {
		return internalServerError(err)
	}

	if err = p.getMetricsSummary(); err != nil {
		return internalServerError(err)
	}

	if err = dataTemplate.ExecuteTemplate(b, "border", p); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func dataSitesPageHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {

	var err error

	if res := checkQuery(r, []string{}, []string{}); !res.ok {
		return res
	}

	// We create a page struct with variables to substitute into the loaded template
	p := dataPage{}
	p.Path = r.URL.Path
	p.Border.Title = "GeoNet MTR"

	if err = p.populateTags(); err != nil {
		return internalServerError(err)
	}

	if err = p.getSitesSummary(); err != nil {
		return internalServerError(err)
	}

	if err = dataTemplate.ExecuteTemplate(b, "border", p); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func getDataSummary() (m map[string]int, err error) {
	u := *mtrApiUrl
	u.Path = "/data/latency/summary"

	var b []byte
	if b, err = getBytes(u.String(), "application/x-protobuf"); err != nil {
		return
	}

	var f mtrpb.DataLatencySummaryResult

	if err = proto.Unmarshal(b, &f); err != nil {
		return
	}

	m = make(map[string]int, 6)
	m["metrics"] = len(f.Result)
	sites := make(map[string]bool)
	for _, r := range f.Result {
		sites[r.SiteID] = true
		incDataCount(m, r)
	}
	m["sites"] = len(sites)
	return
}

func (p *dataPage) getMetricsSummary() (err error) {
	u := *mtrApiUrl
	u.Path = "/data/latency/summary"

	var b []byte
	if b, err = getBytes(u.String(), "application/x-protobuf"); err != nil {
		return
	}

	var f mtrpb.DataLatencySummaryResult

	if err = proto.Unmarshal(b, &f); err != nil {
		return
	}

	p.Metrics = make([]idCount, 0)
	for _, r := range f.Result {
		p.Metrics = updateDataMetric(p.Metrics, r)
	}

	sort.Sort(idCounts(p.Metrics))
	return
}

func (p *dataPage) getSitesSummary() (err error) {
	u := *mtrApiUrl
	u.Path = "/data/latency/summary"

	var b []byte
	if b, err = getBytes(u.String(), "application/x-protobuf"); err != nil {
		return
	}

	var f mtrpb.DataLatencySummaryResult

	if err = proto.Unmarshal(b, &f); err != nil {
		return
	}

	p.Sites = make([]site, 0)
	for _, r := range f.Result {
		p.Sites = updateDataSite(p.Sites, r)
	}

	sort.Sort(sites(p.Sites))
	return
}

// Increase count if Id exists in slice, append to slice if it's a new Id
func updateDataMetric(m []idCount, result *mtrpb.DataLatencySummary) []idCount {
	for _, r := range m {
		if r.Id == result.TypeID {
			incDataCount(r.Count, result)
			return m
		}
	}

	c := make(map[string]int)
	incDataCount(c, result)
	return append(m, idCount{Id: result.TypeID, Count: c})
}

// Increase count if Id exists in slice, append to slice if it's a new Id
func updateDataSite(m []site, result *mtrpb.DataLatencySummary) []site {
	for i, r := range m {
		if r.SiteId == result.SiteID {
			r.TypeCount++
			for j, rt := range r.Types {
				if rt.Id == result.TypeID {
					incDataCount(rt.Count, result)
					r.Types[j] = rt
					m[i] = r
					return m
				}
			}
			// create a new typeId in this SiteId
			r.Types = updateDataMetric(r.Types, result)
			m[i] = r
			return m
		}
	}

	c := make(map[string]int)
	incDataCount(c, result)

	t := []idCount{{Id: result.TypeID, Count: c}}
	return append(m, site{SiteId: result.SiteID, Types: t, TypeCount: 1})
}

func incDataCount(m map[string]int, r *mtrpb.DataLatencySummary) {
	s := getStatusString(r)
	m[s] = m[s] + 1
	m["total"] = m["total"] + 1
}

func getStatusString(r *mtrpb.DataLatencySummary) string {
	switch {
	case r.Upper == 0 && r.Lower == 0:
		return "unknown"
	case allGood(r):
		return "good"
		// TBD: late
	}
	return "bad"
}

func allGood(r *mtrpb.DataLatencySummary) bool {
	if r.Upper == 0 && r.Lower == 0 {
		return false
	}
	if r.Mean < r.Lower || r.Mean > r.Upper {
		return false
	}
	if r.Fifty < r.Lower || r.Fifty > r.Upper {
		return false
	}
	if r.Ninety < r.Lower || r.Ninety > r.Upper {
		return false
	}
	return true
}