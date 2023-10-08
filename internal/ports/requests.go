package ports

import (
	"time"
	"vcr/internal/config"
)

type DeleteProgrammingRequest struct {
	Name string `json:"name"`
}

type GetProgrammingRequest struct {
	Name string `json:"name"`
}

type AddProgrammingRequest struct {
	Url   string `json:"url" yaml:"url" validate:"required,url"`
	Name  string `json:"name" validate:"required"`
	Date  string `json:"start" validate:"required"`
	Until string `json:"end,omitempty" validate:"omitempty"`
}

func (r AddProgrammingRequest) ToProgramming() (config.Programming, error) {
	p := config.Programming{
		Url:   r.Url,
		Name:  r.Name,
		Date:  time.Time{},
		Until: nil,
	}

	var err error
	p.Date, err = parseTime(r.Date, time.Now())
	if err != nil {
		return config.Programming{}, err
	}

	if len(r.Until) == 0 {
		return p, nil
	}

	until, err := parseTime(r.Until, p.Date)
	if err != nil {
		return config.Programming{}, err
	}
	p.Until = &until

	return p, nil
}

func parseTime(input string, relativeBase time.Time) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02T15:04:05", input)
	if err == nil {
		return parsed, nil
	}

	parsed, err = time.Parse("15:04:05", input)
	if err == nil {
		return parsed, nil
	}

	duration, err := time.ParseDuration(input)
	if err == nil {
		return relativeBase.Add(duration), nil
	}

	return time.Time{}, err
}
