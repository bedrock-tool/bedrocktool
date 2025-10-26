package dumpactors

import (
	"encoding/json"

	"github.com/df-mc/dragonfly/server/world/chunk"
)

type ActorsJson struct {
	SourceWorld string  `json:"source_world"`
	Actors      []Actor `json:"actors"`
}

type Actor struct {
	e chunk.Entity
}

type actorJson struct {
	UniqueID   int64                    `json:"unique_id"`
	Identifier string                   `json:"identifier"`
	Data       map[string]*jsonNbtValue `json:"data"`
}

func (a *Actor) MarshalJSON() ([]byte, error) {
	var j actorJson
	j.UniqueID = a.e.ID
	j.Identifier = a.e.Data["identifier"].(string)
	j.Data = make(map[string]*jsonNbtValue)
	for k, v := range a.e.Data {
		j.Data[k] = &jsonNbtValue{Value: v}
	}
	return json.Marshal(j)
}

func (a *Actor) UnmarshalJSON(data []byte) error {
	var j actorJson
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}

	a.e.ID = j.UniqueID
	a.e.Data = make(map[string]any)
	for k, v := range j.Data {
		a.e.Data[k] = v.Value
	}
	a.e.Data["identifier"] = j.Data["identifier"].Value.(string)
	return nil
}
