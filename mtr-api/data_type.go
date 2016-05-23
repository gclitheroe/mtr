package main

import (

	"github.com/GeoNet/weft"
	"fmt"
)

type dataType struct {
	id string
	pk int
	Scale  float64 // used to scale the stored metric for display
	Name   string
	Unit   string // display unit after the metric has been multiplied by scale.
}

var dataTypes = map[string]dataType{
	"latency.strong": {
		pk: 1,
		Scale:  1.0,
		Name:   "latency strong motion data",
		Unit:   "ms",
	},
	"latency.weak": {
		pk: 2,
		Scale:  1.0,
		Name:   "latency weak motion data",
		Unit:   "ms",
	},
	"latency.gnss.1hz": {
		pk: 3,
		Scale:  1.0,
		Name:   "latency GNSS 1Hz data",
		Unit:   "ms",
	},
	"latency.tsunami": {
		pk: 4,
		Scale:  1.0,
		Name:   "latency tsunami data",
		Unit:   "ms",
	},
}

func (d *dataType) read() *weft.Result {
	if d.id == "" {
		return weft.InternalServerError(fmt.Errorf("empty dataType.id"))
	}

	var res *weft.Result
	var t dataType
	if t, res = loadDataType(d.id); !res.Ok {
		return res
	}

	// TODO - do we need to copy the values like this?  Revisit.
	d.pk = t.pk
	d.Scale = t.Scale
	d.Name = t.Name
	d.Unit = t.Unit
	return &weft.StatusOK
}

// TODO combine this into the above func
func loadDataType(typeID string) (dataType, *weft.Result) {
	if f, ok := dataTypes[typeID]; ok {
		return f, &weft.StatusOK
	}

	return dataType{}, weft.BadRequest("invalid type " + typeID)
}
