package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/merge"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/df-mc/goleveldb/leveldb/util"
	"github.com/sirupsen/logrus"
)

type ActorsJson struct {
	SourceWorld string  `json:"source_world"`
	Actors      []Actor `json:"actors"`
}

func formatPath(a string) string {
	a = strings.Trim(a, "\"'")
	a = strings.Map(func(r rune) rune {
		if r == '\\' {
			return '/'
		}
		if r == '"' {
			return -1
		}
		return r
	}, a)
	return a
}

func main() {
	fmt.Printf("%s\n", strings.Join(os.Args, " SPACE "))
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <path to input world folder> [actors.json file] [output world folder]\n", os.Args[0])
		return
	}
	worldPath := os.Args[1]
	worldPath = formatPath(worldPath)

	if len(os.Args) == 3 {
		outputFolder := os.Args[2]
		outputFolder = formatPath(outputFolder)
		doPutIntoWorld(worldPath, outputFolder)
	} else {
		doExtractActors(worldPath)
	}
}

func doExtractActors(worldPath string) {
	inputWorld, err := mcdb.Config{
		Blocks: &merge.BlockRegistry{
			BlockRegistry: world.DefaultBlockRegistry,
			Rids:          make(map[uint32]merge.Block),
		},
		LDBOptions: &opt.Options{
			ReadOnly: true,
		},
	}.Open(worldPath)
	if err != nil {
		logrus.Fatal(err)
	}
	defer inputWorld.Close()

	var actors []Actor
	it := inputWorld.NewColumnIterator(nil)
	defer it.Release()
	for it.Next() {
		c := it.Column()
		if err = it.Error(); err != nil {
			logrus.Fatal(err)
		}

		for _, e := range c.Entities {
			actors = append(actors, Actor{e: e})
			fmt.Printf("%s\n", e.Data["identifier"])
		}
	}

	f, err := os.Create("actors.json")
	if err != nil {
		logrus.Fatal(err)
	}
	defer f.Close()
	je := json.NewEncoder(f)
	je.SetIndent("", "  ")
	if err = je.Encode(ActorsJson{
		SourceWorld: worldPath,
		Actors:      actors,
	}); err != nil {
		logrus.Fatal(err)
	}
}

func doPutIntoWorld(worldPath, outputPath string) {
	if _, err := os.Stat(outputPath); err == nil {
		logrus.Errorf("output path %s exists already", outputPath)
		//return
	}
	os.MkdirAll(outputPath, 0777)

	if err := utils.CopyFS(os.DirFS(worldPath), utils.OSWriter{
		Base: outputPath,
	}); err != nil {
		logrus.Fatal(err)
	}

	outputWorld, err := mcdb.Config{
		Blocks: &merge.BlockRegistry{
			BlockRegistry: world.DefaultBlockRegistry,
			Rids:          make(map[uint32]merge.Block),
		},
		LDBOptions: &opt.Options{
			ReadOnly: true,
		},
	}.Open(outputPath)
	if err != nil {
		logrus.Fatal(err)
	}
	defer outputWorld.Close()

	ldb := outputWorld.LDB()
	removeAllDigp(ldb)
	removeAllActorprefix(ldb)

	f, err := os.Open("actors.json")
	if err != nil {
		logrus.Fatal(err)
	}
	defer f.Close()
	jd := json.NewDecoder(f)
	var aj ActorsJson
	if err := jd.Decode(&aj); err != nil {
		logrus.Fatal(err)
	}

	var actorsByChunk = make(map[world.ChunkPos][]chunk.Entity)
	for _, actor := range aj.Actors {
		pos := actor.e.Data["Pos"].([]float32)
		cp := world.ChunkPos{
			int32(pos[0]) >> 4,
			int32(pos[2]) >> 4,
		}
		actorsByChunk[cp] = append(actorsByChunk[cp], actor.e)
	}
	logrus.Infof("writing actors in %d chunks", len(actorsByChunk))

	var count int
	for cp, actors := range actorsByChunk {
		outputWorld.StoreEntities(cp, world.Overworld, actors)
		count += len(actors)
	}
	logrus.Infof("wrote %d actors", count)
}

func removeAllDigp(ldb *leveldb.DB) {
	var count int
	it := ldb.NewIterator(util.BytesPrefix([]byte("digp")), nil)
	defer it.Release()
	for it.Next() {
		ldb.Delete(it.Key(), nil)
		count++
	}
	err := it.Error()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("removed %d digp entries", count)
}

func removeAllActorprefix(ldb *leveldb.DB) {
	var count int
	it := ldb.NewIterator(util.BytesPrefix([]byte("actorprefix")), nil)
	defer it.Release()
	for it.Next() {
		ldb.Delete(it.Key(), nil)
		count++
	}
	err := it.Error()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("removed %d actors", count)
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

type jsonNbtValue struct {
	Value any
}

// serialized form
type jsonNbtValueInternal struct {
	Uint8   *uint8                          `json:"uint8,omitempty"`
	Int16   *int16                          `json:"int16,omitempty"`
	Int32   *int32                          `json:"int32,omitempty"`
	Int64   *int64                          `json:"int64,omitempty"`
	Float32 *float32                        `json:"float32,omitempty"`
	Float64 *float64                        `json:"float64,omitempty"`
	String  *string                         `json:"string,omitempty"`
	Map     map[string]jsonNbtValueInternal `json:"map,omitempty"`
	Slice   []jsonNbtValueInternal          `json:"slice,omitempty"`
}

func (internal *jsonNbtValueInternal) Value() (any, error) {
	if internal.Uint8 != nil {
		return *internal.Uint8, nil
	} else if internal.Int16 != nil {
		return *internal.Int16, nil
	} else if internal.Int32 != nil {
		return *internal.Int32, nil
	} else if internal.Int64 != nil {
		return *internal.Int64, nil
	} else if internal.Float32 != nil {
		return *internal.Float32, nil
	} else if internal.Float64 != nil {
		return *internal.Float64, nil
	} else if internal.String != nil {
		return *internal.String, nil
	} else if internal.Map != nil {
		m := make(map[string]any)
		for key, val := range internal.Map {
			v, err := val.Value()
			if err != nil {
				return nil, err
			}
			m[key] = v
		}
		return m, nil
	} else if internal.Slice != nil {
		if len(internal.Slice) == 0 {
			return make([]any, 0), nil
		}

		v0, err := internal.Slice[0].Value()
		if err != nil {
			return nil, err
		}

		elemType := reflect.TypeOf(v0)
		sl := reflect.MakeSlice(reflect.SliceOf(elemType), 0, len(internal.Slice))
		for _, e := range internal.Slice {
			v, err := e.Value()
			if err != nil {
				return nil, err
			}
			sl = reflect.Append(sl, reflect.ValueOf(v))
		}
		return sl.Interface(), nil
	} else {
		return nil, nil // Or handle default case if needed
	}
}

func (jv *jsonNbtValue) MarshalJSON() ([]byte, error) {
	internal := jsonNbtValueInternal{}

	switch v := jv.Value.(type) {
	case uint8:
		internal.Uint8 = &v
	case int16:
		internal.Int16 = &v
	case int32:
		internal.Int32 = &v
	case int64:
		internal.Int64 = &v
	case float32:
		internal.Float32 = &v
	case float64:
		internal.Float64 = &v
	case string:
		internal.String = &v
	case map[string]any:
		internal.Map = make(map[string]jsonNbtValueInternal)
		for key, val := range v {
			nested := jsonNbtValue{Value: val}
			nestedInternal := jsonNbtValueInternal{}
			nestedBytes, err := nested.MarshalJSON()
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(nestedBytes, &nestedInternal); err != nil {
				return nil, err
			}
			internal.Map[key] = nestedInternal
		}
	case []any:
		internal.Slice = make([]jsonNbtValueInternal, len(v))
		for i, val := range v {
			nested := jsonNbtValue{Value: val}
			nestedInternal := jsonNbtValueInternal{}
			nestedBytes, err := nested.MarshalJSON()
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(nestedBytes, &nestedInternal); err != nil {
				return nil, err
			}
			internal.Slice[i] = nestedInternal
		}
	default:
		return nil, fmt.Errorf("unsupported type for MarshalJSON: %T", jv.Value)
	}

	return json.Marshal(internal)
}

func (jv *jsonNbtValue) UnmarshalJSON(data []byte) error {
	var internal jsonNbtValueInternal
	if err := json.Unmarshal(data, &internal); err != nil {
		return err
	}

	var err error
	jv.Value, err = internal.Value()
	return err
}
