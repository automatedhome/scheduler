package main

import (
  "fmt"
  "strconv"
  "math"
  "html/template"
  "net/http"
//  "os"
  "flag"
  "time"
  "encoding/json"

  "github.com/eclipse/paho.mqtt.golang"
)

type ScheduleCell struct {
  From string         `json:"from"`
  To string           `json:"to"`
  Temperature float64 `json:"temperature"`
}

type Schedule struct {
  Workday []ScheduleCell     `json:"workday"`
  Freeday []ScheduleCell     `json:"freeday"`
  DefaultTemperature float64 `json:"defaultTemperature"`
}

var MQTTClient mqtt.Client
var MQTTTopic string
var DATA Schedule
var TEMPLATE string

func parseFloat(number string) float64 {
    tmp, _ := strconv.ParseFloat(number, 64)
    return math.Round(tmp*100)/100
}

//func publishData(client mqtt.Client, topic string) {
func publishData() {
    text, _ := json.Marshal(DATA)
    //token := client.Publish(topic, 0, true, string(text))
    token := MQTTClient.Publish(MQTTTopic, 0, true, string(text))
    token.Wait()
    if token.Error() != nil {
    	fmt.Printf("Failed to publish packet: %s", token.Error())
    }
    fmt.Printf("Published packet to topic %s", MQTTTopic)
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

      var workdayCells = []ScheduleCell{} 
      for i := 0; i < len(workdayBegins); i++ {
        t := parseFloat(workdayTemps[i])
        cell := ScheduleCell{
          From: workdayBegins[i],
          To: workdayEnds[i],
          Temperature: t,
        }
        workdayCells = append(workdayCells, cell)
      }
      
      var freedayCells = []ScheduleCell{} 
      for i := 0; i < len(freedayBegins); i++ {
        t := parseFloat(freedayTemps[i])
        cell := ScheduleCell{
          From: freedayBegins[i],
          To: freedayEnds[i],
          Temperature: t,
        }
        freedayCells = append(freedayCells, cell)
      }
      // Parsing ends

      DATA = Schedule{
        DefaultTemperature: defaultTemperature,
        Workday: workdayCells,
        Freeday: freedayCells,
      }
//      publishData(mqttClient, mqttTopic)
      publishData()

    }
    tmpl.Execute(w, DATA)
}

func init() {
  DATA = Schedule{
    DefaultTemperature: 18.0,
    Workday: []ScheduleCell{
      {From: "05:00", To: "06:30", Temperature: 21.1},
      {From: "14:00", To: "21:00", Temperature: 22.2},
    },
    Freeday: []ScheduleCell{
      {From: "07:00", To: "22:00", Temperature: 22.5},
    },
  }
}

func main() {
  server := flag.String("server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
  topic := flag.String("topic", "scheduler", "Topic to subscribe to")
  clientID := flag.String("clientid", "scheduler", "A clientid for the connection")
  address := flag.String("address", ":3000", "Address to expose HTTP interface")
  template := flag.String("template", "/usr/share/site.tmpl", "Path to a site template file")
  flag.Parse()
  
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
  fmt.Printf("Exposing HTTP interface on %s\n", *address)

  http.HandleFunc("/", HTTPHandler)
  http.ListenAndServe(*address, nil)
}
