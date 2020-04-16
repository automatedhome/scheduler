package main

import (
	"fmt"
	"html/template"
	"math"
	"net"
	"net/http"
	"strconv"

	//  "os"
	"encoding/json"
	"flag"
	"strings"
	"time"

	types "github.com/automatedhome/scheduler/pkg/types"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var internalNetwork string
var MQTTClient mqtt.Client
var MQTTTopic string
var DATA types.Schedule
var TEMPLATE string
var REFERERS []string

func getRealAddr(r *http.Request) string {
	remoteIP := ""
	// the default is the originating ip. but we try to find better options because this is almost
	// never the right IP
	if parts := strings.Split(r.RemoteAddr, ":"); len(parts) == 2 {
		remoteIP = parts[0]
	}
	// If we have a forwarded-for header, take the address from there
	if xff := strings.Trim(r.Header.Get("X-Forwarded-For"), ","); len(xff) > 0 {
		addrs := strings.Split(xff, ",")
		lastFwd := addrs[len(addrs)-1]
		if ip := net.ParseIP(lastFwd); ip != nil {
			remoteIP = ip.String()
		}
		// parse X-Real-Ip header
	} else if xri := r.Header.Get("X-Real-Ip"); len(xri) > 0 {
		if ip := net.ParseIP(xri); ip != nil {
			remoteIP = ip.String()
		}
	}

	return remoteIP
}

func parseFloat(number string) float64 {
	tmp, _ := strconv.ParseFloat(number, 64)
	return math.Round(tmp*100) / 100
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

//func publishData(client mqtt.Client, topic string) {
func publishData() {
	text, _ := json.Marshal(DATA)
	//token := client.Publish(topic, 0, true, string(text))
	token := MQTTClient.Publish(MQTTTopic, 0, true, string(text))
	token.Wait()
	if token.Error() != nil {
		fmt.Printf("Failed to publish packet: %s\n", token.Error())
	}
	fmt.Printf("Published packet to topic %s\n", MQTTTopic)
}

func onMessageReceived(client mqtt.Client, message mqtt.Message) {
	fmt.Printf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())
	if err := json.Unmarshal(message.Payload(), &DATA); err != nil {
		fmt.Println("Failed to unmarshal JSON data from MQTT topic. Resending last good values.")
		//publishData(client, message.Topic())
		publishData()
	}
}

func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	ip := getRealAddr(r)
	fmt.Printf("Connected client from %s\n", ip)
	//_, cidr, _ := net.ParseCIDR(internalNetwork)
	//if !cidr.Contains(net.ParseIP(ip)) {
	//	http.Error(w, "403 Access Forbidden", http.StatusForbidden)
	//	return
	//}
	if !stringInSlice(r.Header.Get("Referer"), REFERERS) {
		http.Error(w, "403 Access Forbidden", http.StatusForbidden)
		return
	}

	tmpl := template.Must(template.ParseFiles(TEMPLATE))
	if r.Method == "POST" {
		r.ParseForm()

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

		DATA = types.Schedule{
			DefaultTemperature: defaultTemperature,
			Workday:            workdayCells,
			Freeday:            freedayCells,
		}
		//      publishData(mqttClient, mqttTopic)
		publishData()

	}
	tmpl.Execute(w, DATA)
}

func init() {
	internalNetwork = "192.168.0.0/16"
	DATA = types.Schedule{
		DefaultTemperature: 18.0,
		Workday: []types.ScheduleCell{
			{From: "05:00", To: "06:30", Temperature: 21.1},
			{From: "14:00", To: "21:00", Temperature: 22.2},
		},
		Freeday: []types.ScheduleCell{
			{From: "07:00", To: "22:00", Temperature: 22.5},
		},
	}
}

func main() {
	server := flag.String("server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	topic := flag.String("topic", "thermostat/schedule", "Topic to publish on")
	clientID := flag.String("clientid", "scheduler", "A clientid for the connection")
	address := flag.String("address", ":3000", "Address to expose HTTP interface")
	template := flag.String("template", "/usr/share/site.tmpl", "Path to a site template file")
	tlsCrt := flag.String("tlsCrt", "", "Path to a TLS certificate")
	tlsKey := flag.String("tlsKey", "", "Path to a TLS certificate key")
	referers := flag.String("referers", "", "Comma-separated list of accepted HTTP Referer Headers")
	flag.Parse()

	REFERERS = strings.Split(*referers, ",")

	TEMPLATE = *template
	fmt.Printf("Using template file located at %s\n", *template)

	opts := mqtt.NewClientOptions().AddBroker(*server).SetClientID(*clientID)
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetAutoReconnect(true)

	MQTTTopic = *topic
	opts.OnConnect = func(MQTTCLIENT mqtt.Client) {
		if token := MQTTCLIENT.Subscribe(*topic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	MQTTClient = mqtt.NewClient(opts)
	if token := MQTTClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Printf("Connected to %s as %s and listening on topic: %s\n", *server, *clientID, *topic)

	http.HandleFunc("/", HTTPHandler)
	if *tlsCrt == "" || *tlsKey == "" {
		fmt.Printf("Exposing HTTP interface on %s without TLS config\n", *address)
		http.ListenAndServe(*address, nil)
	} else {
		fmt.Printf("Exposing HTTP interface on %s with TLS config\n", *address)
		http.ListenAndServeTLS(*address, *tlsCrt, *tlsKey, nil)
	}
}
