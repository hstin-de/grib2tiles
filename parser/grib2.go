package parser

/*
#cgo CFLAGS: -I/usr/include/x86_64-linux-gnu/
#cgo LDFLAGS: -leccodes
#include "eccodes.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"math"
	"time"
	"unsafe"
)

type GribHeader struct {
	Type               int32     `json:"type"`
	Nx                 int       `json:"nx"`
	Ny                 int       `json:"ny"`
	La1                float64   `json:"la1"`
	La2                float64   `json:"la2"`
	Lo1                float64   `json:"lo1"`
	Lo2                float64   `json:"lo2"`
	DX                 float64   `json:"dx"`
	DY                 float64   `json:"dy"`
	ScanMode           int       `json:"scanMode"`
	Discipline         int       `json:"discipline"`
	ParameterCategory  int       `json:"parameterCategory"`
	ParameterNumber    int       `json:"parameterNumber"`
	ReferenceTime      time.Time `json:"referenceTime"`
	ForecastTime       int       `json:"forecastTime"`
	EndStep            int       `json:"endStep"`
	MissingValue       float64   `json:"missingValue"`
}

type GRIBFile struct {
	Header     GribHeader
	DataValues []float64
}

func (g GRIBFile) GetLatLng(x, y int) (float64, float64) {
	if x < 0 || x >= g.Header.Nx || y < 0 || y >= g.Header.Ny {
		return 9999, 9999
	}

	lo1 := g.Header.Lo1
	if lo1 > 180 {
		lo1 -= 360
	}

	lat := g.Header.La1 + float64(y)*g.Header.DY
	lng := lo1 + float64(x)*g.Header.DX

	return lat, lng
}

func (g GRIBFile) GetData(lat, lng float64) float64 {
	lo1 := g.Header.Lo1
	if lo1 > 180 {
		lo1 -= 360
	}

	x := int((lng - lo1) / g.Header.DX)
	y := int((lat - g.Header.La1) / g.Header.DY)

	if x < 0 || x >= g.Header.Nx || y < 0 || y >= g.Header.Ny {
		return g.Header.MissingValue
	}

	return g.DataValues[y*g.Header.Nx+x]
}

func (g GRIBFile) GetInterpolatedData(lat, lng float64) float64 {
	la1 := g.Header.La1
	lo1 := g.Header.Lo1
	la2 := g.Header.La2
	lo2 := g.Header.Lo2
	dx := g.Header.DX
	dy := g.Header.DY
	width := g.Header.Nx
	height := g.Header.Ny
	missingValue := g.Header.MissingValue
	data := g.DataValues

	if lo1 > 180 {
		lo1 -= 360
	}
	if lng > 180 {
		lng -= 360
	} else if lng < -180 {
		lng += 360
	}
	
	absDy := math.Abs(dy)
	
	minLat := math.Min(la1, la2)
	maxLat := math.Max(la1, la2)
	minLon := math.Min(lo1, lo2)
	maxLon := math.Max(lo1, lo2)

	if lat < minLat || lat > maxLat || lng < minLon || lng > maxLon {
		return missingValue
	}

	// Calculate fractional grid coordinates directly from lat/lon
	var x, y float64
	if la1 < la2 {
		// Ascending latitudes
		y = (lat - la1) / absDy
	} else {
		// Descending latitudes
		y = (la1 - lat) / absDy
	}
	x = (lng - lo1) / dx

	if x < 0 {
		x = 0
	}
	if x >= float64(width-1) {
		x = float64(width) - 1.001 // Slightly less than width to ensure valid index
	}
	if y < 0 {
		y = 0
	}
	if y >= float64(height-1) {
		y = float64(height) - 1.001 // Slightly less than height to ensure valid index
	}

	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))

	u := x - float64(x0)
	v := y - float64(y0)

	edgeMargin := 3

	if x0 < edgeMargin || x0 >= width-(edgeMargin+1) || y0 < edgeMargin || y0 >= height-(edgeMargin+1) {
		return adaptiveInterpolation(data, x, y, x0, y0, width, height, missingValue, edgeMargin)
	}

	// Calculate bicubic weights
	u2 := u * u
	u3 := u2 * u
	v2 := v * v
	v3 := v2 * v

	wx0 := (-0.5 * u3) + (u2) - (0.5 * u)
	wx1 := (1.5 * u3) - (2.5 * u2) + 1
	wx2 := (-1.5 * u3) + (2.0 * u2) + (0.5 * u)
	wx3 := (0.5 * u3) - (0.5 * u2)
	wy0 := (-0.5 * v3) + (v2) - (0.5 * v)
	wy1 := (1.5 * v3) - (2.5 * v2) + 1
	wy2 := (-1.5 * v3) + (2.0 * v2) + (0.5 * v)
	wy3 := (0.5 * v3) - (0.5 * v2)

	values := make([]float64, 16)
	missingMask := make([]bool, 16)

	indices := [16][2]int{
		{y0 - 1, x0 - 1}, {y0 - 1, x0}, {y0 - 1, x0 + 1}, {y0 - 1, x0 + 2},
		{y0, x0 - 1}, {y0, x0}, {y0, x0 + 1}, {y0, x0 + 2},
		{y0 + 1, x0 - 1}, {y0 + 1, x0}, {y0 + 1, x0 + 1}, {y0 + 1, x0 + 2},
		{y0 + 2, x0 - 1}, {y0 + 2, x0}, {y0 + 2, x0 + 1}, {y0 + 2, x0 + 2},
	}

	hasMissing := false
	for i, idx := range indices {
		row, col := idx[0], idx[1]
		values[i] = data[row*width+col]
		missingMask[i] = (values[i] == missingValue)
		if missingMask[i] {
			hasMissing = true
		}
	}

	if hasMissing {
		return adaptiveInterpolation(data, x, y, x0, y0, width, height, missingValue, edgeMargin)
	}

	weights := []float64{
		wx0 * wy0, wx1 * wy0, wx2 * wy0, wx3 * wy0,
		wx0 * wy1, wx1 * wy1, wx2 * wy1, wx3 * wy1,
		wx0 * wy2, wx1 * wy2, wx2 * wy2, wx3 * wy2,
		wx0 * wy3, wx1 * wy3, wx2 * wy3, wx3 * wy3,
	}

	result := 0.0
	for i := 0; i < 16; i++ {
		result += values[i] * weights[i]
	}

	return result
}

func adaptiveInterpolation(data []float64, x, y float64, x0, y0, width, height int, missingValue float64, edgeMargin int) float64 {
	maxLeft := min(x0, edgeMargin)
	maxRight := min(width-1-x0, edgeMargin)
	maxTop := min(y0, edgeMargin)
	maxBottom := min(height-1-y0, edgeMargin)


	if maxLeft >= 1 && maxRight >= 2 && maxTop >= 1 && maxBottom >= 2 {
		if maxTop >= 1 && maxBottom >= 1 && maxLeft >= 1 && maxRight >= 1 {
			result, valid := tryBilinearInterpolation(data, x, y, x0, y0, width, height, missingValue)
			if valid {
				return result
			}
		}
	}

	return gradientAdaptiveInterpolation(data, x, y, x0, y0, width, height, missingValue)
}

func tryBilinearInterpolation(data []float64, x, y float64, x0, y0, width, height int, missingValue float64) (float64, bool) {
	getPixel := func(row, col int) (float64, bool) {
		if row < 0 || row >= height || col < 0 || col >= width {
			return missingValue, false
		}
		val := data[row*width+col]
		return val, val != missingValue
	}

	u := x - float64(x0)
	v := y - float64(y0)

	v00, valid00 := getPixel(y0, x0)
	v01, valid01 := getPixel(y0, x0+1)
	v10, valid10 := getPixel(y0+1, x0)
	v11, valid11 := getPixel(y0+1, x0+1)

	validCount := 0
	if valid00 {
		validCount++
	}
	if valid01 {
		validCount++
	}
	if valid10 {
		validCount++
	}
	if valid11 {
		validCount++
	}

	if validCount < 2 {
		return 0, false
	}

	if validCount == 4 {
		return v00*(1-u)*(1-v) + v01*u*(1-v) + v10*(1-u)*v + v11*u*v, true
	}

	totalWeight := 0.0
	result := 0.0

	if valid00 {
		weight := (1 - u) * (1 - v)
		result += v00 * weight
		totalWeight += weight
	}
	if valid01 {
		weight := u * (1 - v)
		result += v01 * weight
		totalWeight += weight
	}
	if valid10 {
		weight := (1 - u) * v
		result += v10 * weight
		totalWeight += weight
	}
	if valid11 {
		weight := u * v
		result += v11 * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		return result / totalWeight, true
	}

	return 0, false
}

func gradientAdaptiveInterpolation(data []float64, x, y float64, x0, y0, width, height int, missingValue float64) float64 {
	radius := 3.0

	type DataPoint struct {
		value    float64
		distance float64
	}

	var validPoints []DataPoint

	for dy := -int(radius); dy <= int(radius); dy++ {
		for dx := -int(radius); dx <= int(radius); dx++ {
			nx := x0 + dx
			ny := y0 + dy

			if nx < 0 || nx >= width || ny < 0 || ny >= height {
				continue
			}

			val := data[ny*width+nx]
			if val != missingValue {
				px := float64(nx) + 0.5
				py := float64(ny) + 0.5
				dist := math.Sqrt((px-x)*(px-x) + (py-y)*(py-y))

				if dist <= radius {
					validPoints = append(validPoints, DataPoint{
						value:    val,
						distance: dist,
					})
				}
			}
		}
	}

	if len(validPoints) == 0 {
		return missingValue
	}

	if len(validPoints) == 1 {
		return validPoints[0].value
	}

	var totalWeight float64
	var weightedSum float64

	for _, point := range validPoints {
		weight := 1.0 / (point.distance*point.distance + 0.001)

		weightedSum += point.value * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		return weightedSum / totalWeight
	}

	return missingValue
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ProcessGRIB(gribData []byte) GRIBFile {
	dataPtr := unsafe.Pointer(&gribData[0])
	dataSize := C.size_t(len(gribData))

	var gid *C.codes_handle = C.codes_handle_new_from_message(C.codes_context_get_default(), dataPtr, dataSize)
	if gid == nil {
		fmt.Println("Failed to create handle from message")
		return GRIBFile{}
	}
	defer C.codes_handle_delete(gid)

	// Extract grid information
	var nx, ny C.long
	var la1, la2, lo1, lo2, dx, dy, basicAngle, subdivisions C.double
	var values *C.double
	var numValues C.size_t
	var year, month, day, hour, minute, second, timeUnit, forecastTime, scanMode, endStep C.long

	var discipline, parameterCategory, parameterNumber C.long

	var missingValue C.double

	C.codes_get_long(gid, C.CString("Ni"), &nx)
	C.codes_get_long(gid, C.CString("Nj"), &ny)
	C.codes_get_double(gid, C.CString("latitudeOfFirstGridPointInDegrees"), &la1)
	C.codes_get_double(gid, C.CString("latitudeOfLastGridPointInDegrees"), &la2)
	C.codes_get_double(gid, C.CString("longitudeOfFirstGridPointInDegrees"), &lo1)
	C.codes_get_double(gid, C.CString("longitudeOfLastGridPointInDegrees"), &lo2)
	C.codes_get_double(gid, C.CString("iDirectionIncrement"), &dx)
	C.codes_get_double(gid, C.CString("jDirectionIncrement"), &dy)
	C.codes_get_double(gid, C.CString("basicAngleOfTheInitialProductionDomain"), &basicAngle)
	C.codes_get_double(gid, C.CString("subdivisionsOfBasicAngle"), &subdivisions)

	C.codes_get_double(gid, C.CString("missingValue"), &missingValue)

	scale := 1e6 // Default scale if no basicAngle is defined

	if basicAngle != 0 {
		scale = float64(basicAngle) / float64(subdivisions)
	}

	// Extract reference time
	C.codes_get_long(gid, C.CString("year"), &year)
	C.codes_get_long(gid, C.CString("month"), &month)
	C.codes_get_long(gid, C.CString("day"), &day)
	C.codes_get_long(gid, C.CString("hour"), &hour)
	C.codes_get_long(gid, C.CString("minute"), &minute)
	C.codes_get_long(gid, C.CString("second"), &second)
	C.codes_get_long(gid, C.CString("indicatorOfUnitOfTimeRange"), &timeUnit)
	C.codes_get_long(gid, C.CString("forecastTime"), &forecastTime)
	C.codes_get_long(gid, C.CString("endStep"), &endStep)
	C.codes_get_long(gid, C.CString("scanMode"), &scanMode)
	C.codes_get_long(gid, C.CString("discipline"), &discipline)
	C.codes_get_long(gid, C.CString("parameterCategory"), &parameterCategory)
	C.codes_get_long(gid, C.CString("parameterNumber"), &parameterNumber)

	gribType := int32((discipline & 0xFF) | ((parameterCategory & 0xFF) << 8) | ((parameterNumber & 0xFF) << 16))

	referenceTime := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC)
	var forecastDuration time.Duration

	// Adjust reference time by forecast period
	// https://codes.ecmwf.int/grib/format/grib2/ctables/4/4/
	switch timeUnit {
	case 0: // Minute
		forecastDuration = time.Duration(forecastTime) * time.Minute
	case 1: // Hour
		forecastDuration = time.Duration(forecastTime) * time.Hour
	case 2: // Day
		forecastDuration = time.Duration(forecastTime) * 24 * time.Hour
	case 3: // Month
		forecastDuration = time.Duration(forecastTime) * 24 * time.Hour * 30
	case 4: // Year
		forecastDuration = time.Duration(forecastTime) * 24 * time.Hour * 365
	case 5: // Decade (10 years)
		forecastDuration = time.Duration(forecastTime) * 24 * time.Hour * 365 * 10
	case 6: // Normal (30 years)
		forecastDuration = time.Duration(forecastTime) * 24 * time.Hour * 365 * 30
	case 7: // Century (100 years)
		forecastDuration = time.Duration(forecastTime) * 24 * time.Hour * 365 * 100
	case 10: // 3 hours
		forecastDuration = time.Duration(forecastTime) * 3 * time.Hour
	case 11: // 6 hours
		forecastDuration = time.Duration(forecastTime) * 6 * time.Hour
	case 12: // 12 hours
		forecastDuration = time.Duration(forecastTime) * 12 * time.Hour
	case 13: // Second
		forecastDuration = time.Duration(forecastTime) * time.Second
	case 255: // Missing
		fmt.Println("Forecast time is missing.")
	default:
		fmt.Printf("Unsupported time unit: %d\n", timeUnit)
	}

	forecastReferenceTime := referenceTime.Add(forecastDuration)

	// Getting the values
	if C.codes_get_size(gid, C.CString("values"), &numValues) == C.CODES_SUCCESS {
		values = (*C.double)(C.malloc(numValues * C.sizeof_double))
		defer C.free(unsafe.Pointer(values))

		if C.codes_get_double_array(gid, C.CString("values"), values, &numValues) == C.CODES_SUCCESS {
			dataValues := make([]float64, numValues)
			for i := C.size_t(0); i < numValues; i++ {
				dataValues[i] = float64(*(*C.double)(unsafe.Pointer(uintptr(unsafe.Pointer(values)) + uintptr(i)*uintptr(C.sizeof_double))))
			}

			la1 := float64(la1)
			la2 := float64(la2)
			lo1 := float64(lo1)
			lo2 := float64(lo2)

			dx := float64(dx) / scale
			dy := float64(dy) / scale

			parsedGrib := GRIBFile{
				Header: GribHeader{
					Type:               gribType,
					Nx:                 int(nx),
					Ny:                 int(ny),
					La1:                la1,
					La2:                la2,
					Lo1:                lo1,
					Lo2:                lo2,
					DX:                 dx,
					DY:                 dy,
					ScanMode:           int(scanMode),
					Discipline:         int(discipline),
					ParameterCategory:  int(parameterCategory),
					ParameterNumber:    int(parameterNumber),
					ReferenceTime:      forecastReferenceTime,
					ForecastTime:       int(forecastTime),
					EndStep:            int(endStep),
					MissingValue:       float64(missingValue),
				},
				DataValues: dataValues,
			}

			var correctedDataValues []float64

			offset := int(parsedGrib.Header.DX / 2)

			for y := 0; y < parsedGrib.Header.Ny; y++ {
				for x := 0; x < parsedGrib.Header.Nx; x++ {
					index := y*parsedGrib.Header.Nx + x
					if y%2 == 0 && (index+offset) < len(parsedGrib.DataValues) {
						correctedDataValues = append(correctedDataValues, parsedGrib.DataValues[index+offset])
					} else {
						correctedDataValues = append(correctedDataValues, parsedGrib.DataValues[index])
					}
				}
			}

			parsedGrib.DataValues = correctedDataValues

			return parsedGrib
		}
	}

	return GRIBFile{}
}
