package types

type ScheduleCell struct {
	From        string  `json:"from" yaml:"from"`
	To          string  `json:"to" yaml:"to"`
	Temperature float64 `json:"temperature" yaml:"temperature"`
}

type Schedule struct {
	Workday            []ScheduleCell `json:"workday" yaml:"workday"`
	Freeday            []ScheduleCell `json:"freeday" yaml:"freeday"`
	DefaultTemperature float64        `json:"defaultTemperature" yaml:"default"`
}

type Config struct {
	Schedule Schedule `json:"schedule" yaml:"schedule"`
}

type Mode struct {
	Override bool    `json:"override"`
	Expected float64 `json:"temperature"`
	Holiday  bool    `json:"holiday"`
}
