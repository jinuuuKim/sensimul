package weather

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// KMA ASOS hourly (지상관측 시간자료, kma_sfctm2.php — single time; kma_sfctm3.php
// is the tm1/tm2 range variant with the same column layout) response is
// whitespace delimited text. Lines beginning with '#' are header/metadata; one
// data row is returned per (station, time).
//
// Column layout (0-based) per the live ASOS `help=1` header (fields 1-46):
//
//	#  YYMMDDHHMI STN WD WS GST_WD GST_WS GST_TM PA PS PT PR TA TD HM PV RN ...
//	   idx 0      1   2  3  4      5      6      7  8  9  10 11 12 13 14 15
//
// Confirmed against a real kma_sfctm2.php?help=1 response: TA=12th field (idx 11),
// HM=14th (idx 13), PA=8th (idx 7), WS=4th (idx 3). The fixture test in
// kma_test.go pins these — update both together if a real response differs.
const (
	kmaColTime = 0  // YYMMDDHHMI (KST)
	kmaColSTN  = 1  // station number
	kmaColWD   = 2  // wind direction (16-pt / deg)
	kmaColWS   = 3  // wind speed (m/s)
	kmaColPA   = 7  // local (station) pressure (hPa)
	kmaColTA   = 11 // air temperature (°C)
	kmaColHM   = 13 // relative humidity (%)

	kmaMinCols = 14 // need indices 0..13 present
)

// fetchKMA pulls the latest completed-hour observation for the configured
// station and parses it into a Weather evidence value.
func (c *Client) fetchKMA() (*Weather, error) {
	tm := latestObservationHourKST(c.now())

	endpoint, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid weather base url: %w", err)
	}
	q := endpoint.Query()
	q.Set("tm", tm)
	q.Set("stn", c.Station)
	q.Set("help", "0")
	q.Set("authKey", c.APIKey)
	endpoint.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build kma request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kma request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read kma response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kma returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	w, err := parseKMASfctm(string(body))
	if err != nil {
		return nil, err
	}
	w.FetchedAt = c.now()
	return w, nil
}

// latestObservationHourKST returns the last completed hour in KST formatted as
// YYYYMMDDHHMM (minutes always "00"). ASOS hourly data for the current hour is
// not published immediately, so we step back one hour from now.
func latestObservationHourKST(now time.Time) string {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		// Fallback to a fixed +09:00 offset if tzdata is unavailable.
		loc = time.FixedZone("KST", 9*60*60)
	}
	t := now.In(loc).Add(-time.Hour)
	return fmt.Sprintf("%04d%02d%02d%02d00", t.Year(), int(t.Month()), t.Day(), t.Hour())
}

// parseKMASfctm extracts the temperature/humidity/pressure/wind evidence from a
// kma_sfctm3.php response body. It returns an error if no usable data row exists
// or if a required field (temperature, humidity, pressure) is missing, so the
// caller can retain the previous evidence value instead of using garbage.
func parseKMASfctm(body string) (*Weather, error) {
	var fields []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields = strings.Fields(line)
		break
	}

	if fields == nil {
		return nil, fmt.Errorf("kma response contained no data row")
	}
	if len(fields) < kmaMinCols {
		return nil, fmt.Errorf("kma data row has %d columns, need >= %d", len(fields), kmaMinCols)
	}

	temp, ok := parseTemperature(fields[kmaColTA])
	if !ok {
		return nil, fmt.Errorf("kma temperature (TA=%q) missing or invalid", fields[kmaColTA])
	}
	humidity, ok := parseHumidity(fields[kmaColHM])
	if !ok {
		return nil, fmt.Errorf("kma humidity (HM=%q) missing or invalid", fields[kmaColHM])
	}
	pressure, ok := parsePressure(fields[kmaColPA])
	if !ok {
		return nil, fmt.Errorf("kma pressure (PA=%q) missing or invalid", fields[kmaColPA])
	}
	// Wind is non-critical: default to 0 if missing rather than rejecting the row.
	wind, _ := parseWind(fields[kmaColWS])

	return &Weather{
		TemperatureC: temp,
		HumidityPct:  humidity,
		PressureHPA:  pressure,
		WindSpeedMPS: wind,
		Source:       SourceLive,
	}, nil
}

// isKMAMissing reports whether a value is a KMA "no data" sentinel. Confirmed in
// live responses: gust, precipitation and snow fields show -9 / -9.0 when absent
// (and -99 / -99.0 appears in some datasets). We treat these as missing for every
// field, including temperature. The trade-off: a genuine -9.0°C reading is also
// treated as missing and the previous hour's value is retained — acceptable,
// since the alternative (taking a -9 sentinel as a real -9°C) would jerk the
// ambient far more than one hour of staleness.
func isKMAMissing(v float64) bool {
	return approxEq(v, -9.0) || approxEq(v, -99.0) || v <= -90
}

func approxEq(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// Per-field parsing combines the missing-sentinel check with hard physical bounds.

func parseTemperature(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || isKMAMissing(v) || v < -90 || v > 90 {
		return 0, false
	}
	return v, true
}

func parseHumidity(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || isKMAMissing(v) || v < 0 || v > 100 {
		return 0, false
	}
	return v, true
}

func parsePressure(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || isKMAMissing(v) || v < 800 || v > 1100 {
		return 0, false
	}
	return v, true
}

func parseWind(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || isKMAMissing(v) || v < 0 || v > 120 {
		return 0, false
	}
	return v, true
}
