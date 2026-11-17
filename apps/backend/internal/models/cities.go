package models

import (
	"strings"
	"unicode"
)

// City is a suggested city, with a stable value and names in each UI language.
// Suggestions are deliberately non-exhaustive: other cities remain valid input.
type City struct {
	Value   string     `json:"value"`
	Names   CityNames  `json:"names"`
	Aliases []string   `json:"aliases"`
	Center  CityCenter `json:"center"`
}

// CityCenter is a WGS84 point used to focus a city map, not an administrative boundary.
type CityCenter struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type CityNames struct {
	EN string `json:"en"`
	RU string `json:"ru"`
	KK string `json:"kk"`
}

// Kazakhstan-first suggestions cover the capital, cities of republican
// significance and regional centers. Names were checked against the government
// overview/map and e-government regional-center list; this is not a live registry:
// https://www.gov.kz/uploads/2024/1/4/2f158410978d8140db196e7f8e8ba9f5_original.36943201.pdf
// https://beta.egov.kz/org/maidd/press/article/details/150387?lang=kk
// Map centers are GeoNames populated-place points (PPLA/PPLC), verified against
// https://download.geonames.org/export/dump/KZ.zip on 2026-11-17.
// GeoNames: https://www.geonames.org/ (CC BY 4.0:
// https://creativecommons.org/licenses/by/4.0/). Row comments identify the source
// records; these representative points are not geometric city centroids.
var cityVocabulary = []City{
	// GeoNames 1526384
	{Value: "Almaty", Names: CityNames{EN: "Almaty", RU: "Алматы", KK: "Алматы"}, Aliases: []string{"Alma-Ata", "Алма-Ата"}, Center: CityCenter{Lat: 43.25249, Lon: 76.9115}},
	// GeoNames 1526273
	{Value: "Astana", Names: CityNames{EN: "Astana", RU: "Астана", KK: "Астана"}, Aliases: []string{"Nur-Sultan", "Нур-Султан", "Нұр-Сұлтан"}, Center: CityCenter{Lat: 51.1801, Lon: 71.44598}},
	// GeoNames 1518980
	{Value: "Shymkent", Names: CityNames{EN: "Shymkent", RU: "Шымкент", KK: "Шымкент"}, Aliases: []string{"Chimkent", "Чимкент"}, Center: CityCenter{Lat: 42.30988, Lon: 69.60042}},
	// GeoNames 610612
	{Value: "Aktau", Names: CityNames{EN: "Aktau", RU: "Актау", KK: "Ақтау"}, Aliases: []string{"Aqtau"}, Center: CityCenter{Lat: 43.66105, Lon: 51.17392}},
	// GeoNames 610611
	{Value: "Aktobe", Names: CityNames{EN: "Aktobe", RU: "Актобе", KK: "Ақтөбе"}, Aliases: []string{"Aqtobe", "Aktyubinsk", "Актюбинск"}, Center: CityCenter{Lat: 50.27969, Lon: 57.20718}},
	// GeoNames 610529
	{Value: "Atyrau", Names: CityNames{EN: "Atyrau", RU: "Атырау", KK: "Атырау"}, Aliases: []string{}, Center: CityCenter{Lat: 47.1048, Lon: 51.88427}},
	// GeoNames 609655
	{Value: "Karaganda", Names: CityNames{EN: "Karaganda", RU: "Караганда", KK: "Қарағанды"}, Aliases: []string{"Karagandy", "Qaragandy"}, Center: CityCenter{Lat: 49.80187, Lon: 73.10211}},
	// GeoNames 1522203
	{Value: "Kokshetau", Names: CityNames{EN: "Kokshetau", RU: "Кокшетау", KK: "Көкшетау"}, Aliases: []string{"Kokchetav", "Кокчетав"}, Center: CityCenter{Lat: 53.28414, Lon: 69.39364}},
	// GeoNames 1519948
	{Value: "Konayev", Names: CityNames{EN: "Konayev", RU: "Конаев", KK: "Қонаев"}, Aliases: []string{"Konaev", "Qonaev", "Kapchagay", "Kapshagay", "Капчагай", "Қапшағай"}, Center: CityCenter{Lat: 43.86681, Lon: 77.06304}},
	// GeoNames 1519928
	{Value: "Kostanay", Names: CityNames{EN: "Kostanay", RU: "Костанай", KK: "Қостанай"}, Aliases: []string{"Kostanai", "Qostanai", "Kustanay", "Кустанай"}, Center: CityCenter{Lat: 53.21435, Lon: 63.62463}},
	// GeoNames 1519922
	{Value: "Kyzylorda", Names: CityNames{EN: "Kyzylorda", RU: "Кызылорда", KK: "Қызылорда"}, Aliases: []string{"Qyzylorda", "Kzyl-Orda", "Кзыл-Орда"}, Center: CityCenter{Lat: 44.85278, Lon: 65.50917}},
	// GeoNames 608668
	{Value: "Oral", Names: CityNames{EN: "Oral", RU: "Уральск", KK: "Орал"}, Aliases: []string{"Uralsk"}, Center: CityCenter{Lat: 51.24601, Lon: 51.42558}},
	// GeoNames 1520316
	{Value: "Oskemen", Names: CityNames{EN: "Oskemen", RU: "Усть-Каменогорск", KK: "Өскемен"}, Aliases: []string{"Ust-Kamenogorsk", "Ust Kamenogorsk", "Өскемен"}, Center: CityCenter{Lat: 49.97143, Lon: 82.60586}},
	// GeoNames 1520240
	{Value: "Pavlodar", Names: CityNames{EN: "Pavlodar", RU: "Павлодар", KK: "Павлодар"}, Aliases: []string{}, Center: CityCenter{Lat: 52.27601, Lon: 76.96881}},
	// GeoNames 1520172
	{Value: "Petropavl", Names: CityNames{EN: "Petropavl", RU: "Петропавловск", KK: "Петропавл"}, Aliases: []string{"Petropavlovsk"}, Center: CityCenter{Lat: 54.87343, Lon: 69.15065}},
	// GeoNames 1519422
	{Value: "Semey", Names: CityNames{EN: "Semey", RU: "Семей", KK: "Семей"}, Aliases: []string{"Semipalatinsk", "Семипалатинск"}, Center: CityCenter{Lat: 50.42064, Lon: 80.25025}},
	// GeoNames 1518542
	{Value: "Taldykorgan", Names: CityNames{EN: "Taldykorgan", RU: "Талдыкорган", KK: "Талдықорған"}, Aliases: []string{"Taldyqorgan", "Taldy-Kurgan", "Талды-Курган"}, Center: CityCenter{Lat: 45.01556, Lon: 78.37389}},
	// GeoNames 1516905
	{Value: "Taraz", Names: CityNames{EN: "Taraz", RU: "Тараз", KK: "Тараз"}, Aliases: []string{"Dzhambul", "Zhambyl", "Джамбул", "Жамбыл"}, Center: CityCenter{Lat: 42.89799, Lon: 71.37334}},
	// GeoNames 1517945
	{Value: "Turkistan", Names: CityNames{EN: "Turkistan", RU: "Туркестан", KK: "Түркістан"}, Aliases: []string{"Turkestan"}, Center: CityCenter{Lat: 43.29458, Lon: 68.25685}},
	// GeoNames 1516589
	{Value: "Zhezkazgan", Names: CityNames{EN: "Zhezkazgan", RU: "Жезказган", KK: "Жезқазған"}, Aliases: []string{"Jezkazgan", "Zhezqazghan", "Dzhezkazgan", "Джезказган"}, Center: CityCenter{Lat: 47.79411, Lon: 67.70628}},
}

func cityKey(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func cityMatches(city City, value string) bool {
	key := cityKey(value)
	if key == "" {
		return false
	}
	for _, alias := range append([]string{city.Value, city.Names.EN, city.Names.RU, city.Names.KK}, city.Aliases...) {
		if cityKey(alias) == key {
			return true
		}
	}
	return false
}

// Cities returns independent values so a caller cannot modify the vocabulary.
func Cities() []City {
	cities := append([]City{}, cityVocabulary...)
	for i := range cities {
		cities[i].Aliases = append([]string{}, cities[i].Aliases...)
	}
	return cities
}

// NormalizeCity maps a known name or alias to its stable value. Other city names
// remain usable without extending the suggested vocabulary.
func NormalizeCity(value string) string {
	for _, city := range cityVocabulary {
		if cityMatches(city, value) {
			return city.Value
		}
	}
	return strings.TrimSpace(value)
}

// CityAliases supplies exact names a repository can match against stored city
// text. Case folding and persistence-query mechanics remain with the repository.
func CityAliases(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	for _, city := range cityVocabulary {
		if !cityMatches(city, value) {
			continue
		}
		aliases := []string{city.Value, city.Names.EN, city.Names.RU, city.Names.KK}
		aliases = append(aliases, city.Aliases...)
		aliases = append(aliases, value)
		out := []string{}
		seen := map[string]bool{}
		for _, alias := range aliases {
			key := strings.ToLower(alias)
			if !seen[key] {
				seen[key] = true
				out = append(out, alias)
			}
		}
		return out
	}
	return []string{value}
}
