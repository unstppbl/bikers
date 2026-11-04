package models

import (
	"strings"
	"unicode"
)

// City is a suggested city, with a stable value and names in each UI language.
// Suggestions are deliberately non-exhaustive: other cities remain valid input.
type City struct {
	Value   string    `json:"value"`
	Names   CityNames `json:"names"`
	Aliases []string  `json:"aliases"`
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
var cityVocabulary = []City{
	{Value: "Almaty", Names: CityNames{EN: "Almaty", RU: "Алматы", KK: "Алматы"}, Aliases: []string{"Alma-Ata", "Алма-Ата"}},
	{Value: "Astana", Names: CityNames{EN: "Astana", RU: "Астана", KK: "Астана"}, Aliases: []string{"Nur-Sultan", "Нур-Султан", "Нұр-Сұлтан"}},
	{Value: "Shymkent", Names: CityNames{EN: "Shymkent", RU: "Шымкент", KK: "Шымкент"}, Aliases: []string{"Chimkent", "Чимкент"}},
	{Value: "Aktau", Names: CityNames{EN: "Aktau", RU: "Актау", KK: "Ақтау"}, Aliases: []string{"Aqtau"}},
	{Value: "Aktobe", Names: CityNames{EN: "Aktobe", RU: "Актобе", KK: "Ақтөбе"}, Aliases: []string{"Aqtobe", "Aktyubinsk", "Актюбинск"}},
	{Value: "Atyrau", Names: CityNames{EN: "Atyrau", RU: "Атырау", KK: "Атырау"}, Aliases: []string{}},
	{Value: "Karaganda", Names: CityNames{EN: "Karaganda", RU: "Караганда", KK: "Қарағанды"}, Aliases: []string{"Karagandy", "Qaragandy"}},
	{Value: "Kokshetau", Names: CityNames{EN: "Kokshetau", RU: "Кокшетау", KK: "Көкшетау"}, Aliases: []string{"Kokchetav", "Кокчетав"}},
	{Value: "Konayev", Names: CityNames{EN: "Konayev", RU: "Конаев", KK: "Қонаев"}, Aliases: []string{"Konaev", "Qonaev", "Kapchagay", "Kapshagay", "Капчагай", "Қапшағай"}},
	{Value: "Kostanay", Names: CityNames{EN: "Kostanay", RU: "Костанай", KK: "Қостанай"}, Aliases: []string{"Kostanai", "Qostanai", "Kustanay", "Кустанай"}},
	{Value: "Kyzylorda", Names: CityNames{EN: "Kyzylorda", RU: "Кызылорда", KK: "Қызылорда"}, Aliases: []string{"Qyzylorda", "Kzyl-Orda", "Кзыл-Орда"}},
	{Value: "Oral", Names: CityNames{EN: "Oral", RU: "Уральск", KK: "Орал"}, Aliases: []string{"Uralsk"}},
	{Value: "Oskemen", Names: CityNames{EN: "Oskemen", RU: "Усть-Каменогорск", KK: "Өскемен"}, Aliases: []string{"Ust-Kamenogorsk", "Ust Kamenogorsk", "Өскемен"}},
	{Value: "Pavlodar", Names: CityNames{EN: "Pavlodar", RU: "Павлодар", KK: "Павлодар"}, Aliases: []string{}},
	{Value: "Petropavl", Names: CityNames{EN: "Petropavl", RU: "Петропавловск", KK: "Петропавл"}, Aliases: []string{"Petropavlovsk"}},
	{Value: "Semey", Names: CityNames{EN: "Semey", RU: "Семей", KK: "Семей"}, Aliases: []string{"Semipalatinsk", "Семипалатинск"}},
	{Value: "Taldykorgan", Names: CityNames{EN: "Taldykorgan", RU: "Талдыкорган", KK: "Талдықорған"}, Aliases: []string{"Taldyqorgan", "Taldy-Kurgan", "Талды-Курган"}},
	{Value: "Taraz", Names: CityNames{EN: "Taraz", RU: "Тараз", KK: "Тараз"}, Aliases: []string{"Dzhambul", "Zhambyl", "Джамбул", "Жамбыл"}},
	{Value: "Turkistan", Names: CityNames{EN: "Turkistan", RU: "Туркестан", KK: "Түркістан"}, Aliases: []string{"Turkestan"}},
	{Value: "Zhezkazgan", Names: CityNames{EN: "Zhezkazgan", RU: "Жезказган", KK: "Жезқазған"}, Aliases: []string{"Jezkazgan", "Zhezqazghan", "Dzhezkazgan", "Джезказган"}},
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
