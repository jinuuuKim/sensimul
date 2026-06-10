package weather

import (
	"math"
	"testing"
	"time"
)

// representativeBody is a REAL kma_sfctm2.php response (station 108, 2026-06-10
// 09:00 KST). Columns: TM STN WD WS GST_WD GST_WS GST_TM PA PS PT PR TA TD HM ...
// Parsed evidence: TA=20.9°C HM=72.0% PA=999.1hPa WS=2.2m/s.
const representativeBody = `#START7777
# YYMMDDHHMI STN  WD   WS GST  GST  GST     PA     PS PT    PR    TA    TD    HM    PV     RN
202606100900 108  18  2.2  -9 -9.0   -9  999.1 1009.0  1   0.3  20.9  15.6  72.0  17.7   -9.0    0.6    0.6   -9.0   -9.0   -9.0   -9.0 -9 62 -                        5   5    6 -         -9  -9  -9  1389  0.3  1.22 -9  25.1  21.1  21.5  21.6  22.1  -9 -9.0 -9  3  1
#7777END`

func TestParseKMASfctm(t *testing.T) {
	w, err := parseKMASfctm(representativeBody)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if math.Abs(w.TemperatureC-20.9) > 1e-9 {
		t.Errorf("temperature = %v, want 20.9", w.TemperatureC)
	}
	if math.Abs(w.HumidityPct-72.0) > 1e-9 {
		t.Errorf("humidity = %v, want 72.0", w.HumidityPct)
	}
	if math.Abs(w.PressureHPA-999.1) > 1e-9 {
		t.Errorf("pressure = %v, want 999.1", w.PressureHPA)
	}
	if math.Abs(w.WindSpeedMPS-2.2) > 1e-9 {
		t.Errorf("wind = %v, want 2.2", w.WindSpeedMPS)
	}
	if w.Source != SourceLive {
		t.Errorf("source = %v, want live", w.Source)
	}
}

func TestParseKMASfctmRealNegativeTemperatureValid(t *testing.T) {
	// -12.3°C is a real sub-zero reading (not a -9/-99 sentinel) → accepted.
	body := ` 202601101000 108 18 2.2 -9 -9.0 -9 1015.0 1018.0 1 -2.0 -12.3 -15.6 55.0 3.0 0.0`
	w, err := parseKMASfctm(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if math.Abs(w.TemperatureC-(-12.3)) > 1e-9 {
		t.Errorf("temperature = %v, want -12.3", w.TemperatureC)
	}
}

func TestParseKMASfctmSentinelTemperatureRejected(t *testing.T) {
	// -9.0 is the KMA missing sentinel (confirmed in live data) → rejected.
	body := ` 202601101000 108 18 2.2 -9 -9.0 -9 1015.0 1018.0 1 -2.0 -9.0 -15.6 55.0 3.0 0.0`
	if _, err := parseKMASfctm(body); err == nil {
		t.Fatal("expected error for missing temperature sentinel -9.0")
	}
}

func TestParseKMASfctmMissingTemperatureRejected(t *testing.T) {
	body := ` 202506101000 108 18 2.2 -9 -9.0 -9 1008.7 1012.3 1 -1.2 -99.0 18.3 67.0 21.1 0.0`
	if _, err := parseKMASfctm(body); err == nil {
		t.Fatal("expected error for missing temperature sentinel -99.0")
	}
}

func TestParseKMASfctmMissingHumidityRejected(t *testing.T) {
	body := ` 202506101000 108 18 2.2 -9 -9.0 -9 1008.7 1012.3 1 -1.2 24.5 18.3 -9.0 21.1 0.0`
	if _, err := parseKMASfctm(body); err == nil {
		t.Fatal("expected error for missing humidity sentinel -9.0")
	}
}

func TestParseKMASfctmNoDataRow(t *testing.T) {
	body := "#  header only\n#7777END"
	if _, err := parseKMASfctm(body); err == nil {
		t.Fatal("expected error when response has no data row")
	}
}

// NOTE: the kma_pm10.php layout is not yet verified against a live response;
// pmColumn is configurable for exactly that reason. This fixture pins the
// parser's mechanics (skip headers, pick column, reject sentinels), not the
// real column index.
const pm10Body = `#START7777
# TM           STN  PM10
 202606100900  108  47
#7777END`

func TestParsePM10(t *testing.T) {
	v, err := parsePM10(pm10Body, 2)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v != 47 {
		t.Fatalf("pm10 = %v, want 47", v)
	}
}

func TestParsePM10MissingRejected(t *testing.T) {
	body := ` 202606100900 108 -9.0`
	if _, err := parsePM10(body, 2); err == nil {
		t.Fatal("expected error for -9.0 sentinel PM10")
	}
}

func TestParsePM10ColumnOutOfRange(t *testing.T) {
	if _, err := parsePM10(` 202606100900 108 47`, 9); err == nil {
		t.Fatal("expected error when column index exceeds row width")
	}
}

func TestLatestObservationHourKST(t *testing.T) {
	// 2025-06-10 01:30 UTC == 10:30 KST → last completed hour 09:00 KST.
	now := time.Date(2025, 6, 10, 1, 30, 0, 0, time.UTC)
	got := latestObservationHourKST(now)
	if got != "202506100900" {
		t.Fatalf("latestObservationHourKST = %s, want 202506100900", got)
	}
}
