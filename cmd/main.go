package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"flag"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"

	types "github.com/automatedhome/scheduler/pkg/types"
)

var (
	TEMPLATE    string
	TOKEN       string
	lastPass    time.Time
	config      types.Config
	stateFile   string
	overrideEnd time.Time
	mode        types.Mode
)

var (
	expectedTemperature = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "thermostat_expected_temperature",
		Help: "Current expected temperature",
	})
	overrideTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "thermostat_override_total",
		Help: "Total number of manual overrides",
	})
)

func parseFloat(number string) float64 {
	tmp, _ := strconv.ParseFloat(number, 64)
	return math.Round(tmp*100) / 100
}

func dumpConfig() {
	d, err := yaml.Marshal(&config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = ioutil.WriteFile(stateFile, d, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func stringToDate(str string) time.Time {
	now := time.Now()
	t := strings.Split(str, ":")
	h, _ := strconv.Atoi(t[0])
	m, _ := strconv.Atoi(t[1])
	return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.Local)
}

func httpSchedule(w http.ResponseWriter, r *http.Request) {
	params, ok := r.URL.Query()["token"]
	if !ok || len(params[0]) < 1 {
		fmt.Println("No token received")
		http.Error(w, "403 Access Forbidden", http.StatusForbidden)
		return
	}
	token := string(params[0])
	if token != TOKEN {
		fmt.Printf("Received incorrect token: %s\n", token)
		http.Error(w, "403 Access Forbidden", http.StatusForbidden)
		return
	}

	tmpl := template.Must(template.ParseFiles(TEMPLATE))
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Printf("Cannot parse form: %v\n", err)
			http.Error(w, "Incorrect form", http.StatusBadRequest)
			return
		}

		// Parsing Form.
		// TODO: Convert Form data to JSON on a client and unmarshal it here to correct data structure
		defaultTemperature := parseFloat(r.FormValue("defaultTemperature"))
		workdayBegins := r.Form["workdayFrom"]
		workdayEnds := r.Form["workdayTo"]
		workdayTemps := r.Form["workdayTemperature"]
		freedayBegins := r.Form["freedayFrom"]
		freedayEnds := r.Form["freedayTo"]
		freedayTemps := r.Form["freedayTemperature"]

		var workdayCells = []types.ScheduleCell{}
		for i := 0; i < len(workdayBegins); i++ {
			t := parseFloat(workdayTemps[i])
			cell := types.ScheduleCell{
				From:        workdayBegins[i],
				To:          workdayEnds[i],
				Temperature: t,
			}
			workdayCells = append(workdayCells, cell)
		}

		var freedayCells = []types.ScheduleCell{}
		for i := 0; i < len(freedayBegins); i++ {
			t := parseFloat(freedayTemps[i])
			cell := types.ScheduleCell{
				From:        freedayBegins[i],
				To:          freedayEnds[i],
				Temperature: t,
			}
			freedayCells = append(freedayCells, cell)
		}
		// Parsing ends

		config.Schedule = types.Schedule{
			DefaultTemperature: defaultTemperature,
			Workday:            workdayCells,
			Freeday:            freedayCells,
		}

		dumpConfig()
	}
	err := tmpl.Execute(w, config.Schedule)
	if err != nil {
		log.Printf("Error templating config: %v\n", err)
		http.Error(w, "Templating error", http.StatusInternalServerError)
		return
	}
}

func httpConfig(w http.ResponseWriter, r *http.Request) {
	js, err := json.Marshal(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		log.Printf("Error sending config values: %v\n", err)
	}
}

func httpHealthCheck(w http.ResponseWriter, r *http.Request) {
	timeout := time.Duration(1 * time.Minute)
	if lastPass.Add(timeout).After(time.Now()) {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(500)
	}
}

func httpHoliday(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := json.NewDecoder(r.Body).Decode(&mode.Holiday); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	} else {
		v := strconv.FormatBool(mode.Holiday)
		_, err := w.Write([]byte(v))
		if err != nil {
			log.Printf("Error sending holiday mode: %v\n", err)
		}
	}
}

func httpExpectedTemp(w http.ResponseWriter, r *http.Request) {
	old := mode.Expected
	if err := json.NewDecoder(r.Body).Decode(&mode.Expected); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if old != mode.Expected {
		mode.Override = true
		overrideTotal.Inc()
		overrideEnd = time.Now().Add(time.Hour) // TODO: make override time configurable and read from config file
	}
}

func httpOverrideMode(w http.ResponseWriter, r *http.Request) {
	if err := json.NewDecoder(r.Body).Decode(&mode.Override); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if mode.Override {
		overrideTotal.Inc()
		overrideEnd = time.Now().Add(time.Hour) // TODO: make override time configurable and read from config file
	} else {
		overrideEnd = time.Now()
	}
}

func httpOperationMode(w http.ResponseWriter, r *http.Request) {
	rsp, err := json.Marshal(mode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(rsp)
	if err != nil {
		log.Printf("Error sending operation mode: %v\n", err)
	}
}

func init() {
	template := flag.String("template", "site.html", "Path to a site template file")
	authtoken := flag.String("token", "", "Auth token")
	configFile := flag.String("config", "config.yaml", "Provide configuration file")
	dataDir := flag.String("data", "/var/lib/thermostat", "Data directory")
	flag.Parse()

	TOKEN = *authtoken
	if TOKEN == "" {
		panic("Missing auth token")
	}

	stateFile = *dataDir + "/state.yaml"

	var cfg string
	if _, err := os.Stat(stateFile); err == nil {
		cfg = stateFile
	} else {
		cfg = *configFile
	}

	log.Printf("Reading configuration from %s", cfg)
	data, err := ioutil.ReadFile(cfg)
	if err != nil {
		log.Fatalf("File reading error: %v", err)
		return
	}

	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		log.Fatalf("error: %v", err)
	}
	log.Printf("Reading following config from config file: %#v", config)

	TEMPLATE = *template
	fmt.Printf("Using template file located at %s\n", *template)

	mode.Holiday = false
	mode.Override = false
	mode.Expected = 0
}

func main() {
	// Expose metrics
	http.Handle("/metrics", promhttp.Handler())
	// Expose config
	http.HandleFunc("/config", httpConfig)
	// Expose healthcheck
	http.HandleFunc("/health", httpHealthCheck)
	// override settings
	http.HandleFunc("/mode", httpOperationMode)
	http.HandleFunc("/mode/holiday", httpHoliday)
	http.HandleFunc("/mode/override", httpOverrideMode)
	http.HandleFunc("/mode/expected", httpExpectedTemp)
	// Expose schedule
	http.HandleFunc("/schedule", httpSchedule)
	go func() {
		if err := http.ListenAndServe(":7009", nil); err != nil {
			panic("HTTP Server failed: " + err.Error())
		}
	}()

	for {
		time.Sleep(5 * time.Second)
		lastPass = time.Now()

		// Reset override temperature to 0 when override period expires
		if time.Now().After(overrideEnd) {
			mode.Override = false
		}

		// check if manual override heating mode is enabled
		if mode.Override {
			expectedTemperature.Set(mode.Expected)
			continue
		}

		// check if now is the time to start heating
		cells := config.Schedule.Workday
		if mode.Holiday {
			cells = config.Schedule.Freeday
		}

		temp := config.Schedule.DefaultTemperature
		for _, cell := range cells {
			from := stringToDate(cell.From)
			to := stringToDate(cell.To)
			if time.Now().After(from) && time.Now().Before(to) {
				temp = cell.Temperature
				continue
			}
		}

		mode.Expected = temp
		expectedTemperature.Set(temp)
	}
}
