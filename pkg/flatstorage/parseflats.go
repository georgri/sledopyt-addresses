package flatstorage

import (
	"encoding/json"
	"fmt"
	"github.com/georgri/sledopyt_addresses/pkg/util"
	"sort"
	"strings"
	"time"
)

const FlatValidInterval = 60 * time.Minute

// url example: https://flat.pik-service.ru/api/v1/filter/flat-by-block/1240?type=1,2&location=2,3&flatLimit=80&onlyFlats=1
// source example:
// {"id":830713,"area":65.2,"floor":17,"metro":{"id":148,
// "name":"\u041d\u0430\u0433\u0430\u0442\u0438\u043d\u0441\u043a\u0430\u044f","color":"#ACADAF"},
// "price":21796360,"rooms":2,"status":"free","typeId":1,
// "planUrl":"https:\/\/0.db-estate.cdn.pik-service.ru\/layout\/2022\/06\/13\/1_sem2_2el36_4_2x12_6-1_t_a_90_PgbXHE4ZDppCmmc2.svg",
// "bulkName":"\u041a\u043e\u0440\u043f\u0443\u0441 1.1","maxFloor":33,
// "blockName":"\u0412\u0442\u043e\u0440\u043e\u0439 \u041d\u0430\u0433\u0430\u0442\u0438\u043d\u0441\u043a\u0438\u0439",
// "blockSlug":"2ngt","finishType":1,"meterPrice":334300,"settlementDate":"2025-06-15","currentBenefitId":114464}
type Flat struct {
	ID     int64   `json:"id"`
	Area   float64 `json:"area"`
	Floor  int64   `json:"floor"`
	Metro  Metro   `json:"metro"`
	Price  int64   `json:"price"`
	Rooms  int8    `json:"rooms"`
	Status string  `json:"status"`
	// TODO: find Url plan with areas, maybe with address https://www.pik.ru/flat/819556
	// https://flat.pik-service.ru/api/v1/flat/819556
	PlanURL   string `json:"planUrl"`   // https:\/\/0.db-estate.cdn.pik-service.ru\/layout\/2022\/06\/13\/1_sem2_2el36_4_2x12_6-1_t_a_90_PgbXHE4ZDppCmmc2.svg
	BulkName  string `json:"bulkName"`  // Корпус 1.1
	MaxFloor  int8   `json:"maxFloor"`  // 33
	BlockName string `json:"blockName"` // Второй Нагатинский
	BlockSlug string `json:"blockSlug"`
	Created   string `json:"created,omitempty"` // when the flat first appeared
	Updated   string `json:"updated,omitempty"` // when the flat was last seen (to filter out the old ones)
}

type MessageData struct {
	Flats []Flat `json:"flats"`

	LastPage int
}

type Metro struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Body struct {
	Data Data `json:"data"`
}

type Stats struct {
	LastPage int `json:"lastPage"`
}

type Data struct {
	Items []Flat `json:"items"`
	Stats Stats  `json:"stats"`
}

func UnmarshallFlats(body []byte) (*MessageData, error) {
	unmarshalled := &Body{}
	err := json.Unmarshal(body, unmarshalled)
	if err != nil {
		return nil, err
	}

	res := &MessageData{
		Flats: unmarshalled.Data.Items,

		LastPage: unmarshalled.Data.Stats.LastPage,
	}

	return res, nil
}

func (md *MessageData) Copy() *MessageData {
	if md == nil {
		return nil
	}
	newMsg := &MessageData{LastPage: md.LastPage, Flats: make([]Flat, 0, len(md.Flats))}
	for _, flat := range md.Flats {
		newMsg.Flats = append(newMsg.Flats, flat)
	}
	return newMsg
}

// String print in human readable telegram friendly format
// example input:
// {831859 32.6 19 {Нагатинская #ACADAF} 12756380 1 free
// https://0.db-estate.cdn.pik-service.ru/attachment/0/167b4389-02d9-eb11-84e9-02bf0a4d8e27/6_sem2_1es3_5.7-1_s_z_07ef74f33ec511c288fe633c87ef297c.svg
// Корпус 1.3 33 Второй Нагатинский}
// example output:
// {number of Flats} новых объектов в ЖК "Второй Нагатинский" (м.Нагатинская (color #ACADAF)):
// Корпус 1.3 #831859[url link to flat]: 32.6m, 1r, f19, 12_756_380rub,
func (md *MessageData) String() string {

	// sorting by price
	sort.Slice(md.Flats, func(i, j int) bool {
		return md.Flats[i].Price < md.Flats[j].Price
	})

	res := md.MakeHeader()

	flats := make([]string, 0, len(md.Flats))
	for _, flat := range md.Flats {
		flats = append(flats, flat.String())
	}

	res += "\n" + strings.Join(flats, "\n") // try <br>

	return res
}

// MakeHeader example:
// // {number of Flats} новых объектов в ЖК "Второй Нагатинский" (м.Нагатинская (color #ACADAF)):
func (md *MessageData) MakeHeader() string {

	if md == nil || len(md.Flats) == 0 {
		return ""
	}

	flat := md.Flats[0]
	numFlats := len(md.Flats)
	blockName := flat.BlockName
	// metro := flat.Metro.Name // to large message
	// metroColor := flat.Metro.Color // telegram doesn't support text color :(

	res := fmt.Sprintf("%v new flats in %v:",
		numFlats, blockName)

	return res
}

func (md *MessageData) GetBlockSlug() string {
	if md == nil || len(md.Flats) == 0 {
		return ""
	}
	return md.Flats[0].BlockSlug
}

func (f *Flat) RecentlyUpdated(now time.Time) bool {
	t, err := time.Parse(time.RFC3339, f.Updated)
	if err != nil {
		return false
	}
	return now.Sub(t) < FlatValidInterval
}

// String example:
// Корпус 1.3 #831859[url link to flat]: 32.6m, 1r, f19, 12_756_380rub,
func (f *Flat) String() string {
	if f == nil {
		return ""
	}

	corp := strings.Split(f.BulkName, " ")[1]
	id := f.ID
	flatURL := fmt.Sprintf("https://www.pik.ru/flat/%v", id)
	area := fmt.Sprintf("%.1f", f.Area)
	rooms := f.Rooms
	floor := f.Floor
	price := util.ThousandSep(f.Price, " ")
	var reserve string
	if f.Status == "reserve" {
		reserve = "🔒"
	}

	res := fmt.Sprintf("%v: <a href=\"%v\">%vr, %vm2</a>, %vR, f%v%v", corp, flatURL, rooms, area, price, floor, reserve)

	return res
}
