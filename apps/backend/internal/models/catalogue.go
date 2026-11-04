package models

import (
	"strings"
	"unicode"
)

// This intentionally small, curated vocabulary provides lookup suggestions;
// owners can enter makes/models that are absent from it as freeform values.
var bikeModelVocabulary = []BikeModel{
	{ID: "honda-cb500x", Make: "Honda", Model: "CB500X", Aliases: []string{"Honda CB 500 X", "Хонда", "Хонда CB500X"}, Images: []BikeModelImage{{URL: "/bike-models/honda-cb500x-2016.jpg", YearFrom: 2016, YearTo: 2016, Width: 960, Height: 720, SourceURL: "https://commons.wikimedia.org/wiki/File:2016-Honda-CB500X-right.jpg", Author: "Blacktav", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}, {URL: "/bike-models/honda-cb500x-2019.jpg", YearFrom: 2019, YearTo: 2019, Width: 960, Height: 1019, SourceURL: "https://commons.wikimedia.org/wiki/File:2019_Honda_CB500X.jpg", Author: "Lancia Dude", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "yamaha-mt-07", Make: "Yamaha", Model: "MT-07", Aliases: []string{"Yamaha MT07", "Ямаха", "Ямаха МТ07"}, Images: []BikeModelImage{{URL: "/bike-models/yamaha-mt-07-2021.jpg", YearFrom: 2021, YearTo: 2021, Width: 960, Height: 603, SourceURL: "https://commons.wikimedia.org/wiki/File:2021_Black_Yamaha_MT-07.jpg", Author: "PackMecEng", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}, {URL: "/bike-models/yamaha-mt-07-2025.jpg", YearFrom: 2025, YearTo: 2025, Width: 960, Height: 982, SourceURL: "https://commons.wikimedia.org/wiki/File:Yamaha_MT-07_2025_EICMA_2024.jpg", Author: "AVMOTO", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "kawasaki-ninja-400", Make: "Kawasaki", Model: "Ninja 400", Aliases: []string{"Kawasaki Ninja400", "Кавасаки", "Кавасаки Ниндзя 400"}, Images: []BikeModelImage{{URL: "/bike-models/kawasaki-ninja-400-2018.jpg", YearFrom: 2018, YearTo: 2018, Width: 2000, Height: 1404, SourceURL: "https://commons.wikimedia.org/wiki/File:Kawasaki_Ninja_400.jpg", Author: "Rikita", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "suzuki-sv650", Make: "Suzuki", Model: "SV650", Aliases: []string{"Suzuki SV 650", "Сузуки", "Сузуки СВ650"}, Images: []BikeModelImage{{URL: "/bike-models/suzuki-sv650-2004.jpg", YearFrom: 2004, YearTo: 2004, Width: 1280, Height: 960, SourceURL: "https://commons.wikimedia.org/wiki/File:SuzukiSV650.jpg", Author: "Trondos", License: "Public domain", LicenseURL: "https://commons.wikimedia.org/wiki/File:SuzukiSV650.jpg"}}},
	{ID: "bmw-r1250gs", Make: "BMW", Model: "R 1250 GS", Aliases: []string{"BMW R1250GS", "БМВ", "БМВ R1250GS"}, Images: []BikeModelImage{}},
	{ID: "ktm-390-duke", Make: "KTM", Model: "390 Duke", Aliases: []string{"KTM Duke390", "КТМ", "КТМ 390 Дюк"}, Images: []BikeModelImage{{URL: "/bike-models/ktm-390-duke-2015.jpg", YearFrom: 2015, YearTo: 2015, Width: 960, Height: 633, SourceURL: "https://commons.wikimedia.org/wiki/File:2015_KTM_390_Duke_left.JPG", Author: "Dennis Bratland", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}, {URL: "/bike-models/ktm-390-duke-2018.jpg", YearFrom: 2018, YearTo: 2018, Width: 1921, Height: 1467, SourceURL: "https://commons.wikimedia.org/wiki/File:Ktm_duke_390.jpg", Author: "28pg", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "yamaha-tenere-700", Make: "Yamaha", Model: "Ténéré 700", Aliases: []string{"Yamaha Tenere 700", "Yamaha T7", "Ямаха Тенере 700", "Ямаха Тэнэрэ 700"}, Images: []BikeModelImage{{URL: "/bike-models/yamaha-tenere-700-2021.jpg", YearFrom: 2021, YearTo: 2021, Width: 960, Height: 720, SourceURL: "https://commons.wikimedia.org/wiki/File:Tenere_700.jpg", Author: "Happypap55", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "yamaha-mt-09", Make: "Yamaha", Model: "MT-09", Aliases: []string{"Yamaha MT09", "Yamaha MT 09", "Ямаха МТ-09", "Ямаха МТ09"}, Images: []BikeModelImage{{URL: "/bike-models/yamaha-mt-09-2024.jpg", YearFrom: 2024, YearTo: 2024, Width: 960, Height: 720, SourceURL: "https://commons.wikimedia.org/wiki/File:2024_Yamaha_MT-09.jpg", Author: "Chanokchon", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "yamaha-yzf-r3", Make: "Yamaha", Model: "YZF-R3", Aliases: []string{"Yamaha R3", "Yamaha YZFR3", "Ямаха Р3", "Ямаха YZF-R3"}, Images: []BikeModelImage{{URL: "/bike-models/yamaha-yzf-r3-2015.jpg", YearFrom: 2015, YearTo: 2015, Width: 960, Height: 633, SourceURL: "https://commons.wikimedia.org/wiki/File:2015_Yamaha_YZF-R3_right.JPG", Author: "Dennis Bratland", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "honda-crf1000l-africa-twin-mt-abs", Make: "Honda", Model: "CRF1000L Africa Twin (ABS, MT)", Aliases: []string{"Honda CRF1000L Africa Twin ABS manual", "Honda Africa Twin 1000 MT ABS", "Хонда Африка Твин 1000 ABS механика", "Хонда CRF1000L"}, Images: []BikeModelImage{{URL: "/bike-models/honda-crf1000l-africa-twin-mt-abs-2016.jpg", YearFrom: 2016, YearTo: 2016, Width: 960, Height: 641, SourceURL: "https://commons.wikimedia.org/wiki/File:Honda_CRF1000L.jpg", Author: "Matteo Casarico", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "honda-crf1100l-africa-twin-mt", Make: "Honda", Model: "CRF1100L Africa Twin MT", Aliases: []string{"Honda Africa Twin 1100 MT", "Honda CRF 1100 L manual", "Хонда Африка Твин 1100 механика", "Хонда CRF1100L MT"}, Images: []BikeModelImage{{URL: "/bike-models/honda-crf1100l-africa-twin-mt-2020.jpg", YearFrom: 2020, YearTo: 2020, Width: 960, Height: 960, SourceURL: "https://commons.wikimedia.org/wiki/File:2020_Honda_CRF1100L_Africa_Twin_MT.jpg", Author: "Chanokchon", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "honda-cb650r", Make: "Honda", Model: "CB650R", Aliases: []string{"Honda CB 650 R", "Honda CB650 R", "Хонда СБ650Р", "Хонда CB650R"}, Images: []BikeModelImage{{URL: "/bike-models/honda-cb650r-2019.jpg", YearFrom: 2019, YearTo: 2019, Width: 960, Height: 926, SourceURL: "https://commons.wikimedia.org/wiki/File:2019_Honda_CB650R_(20190428).jpg", Author: "オーバードライブ83", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "kawasaki-ninja-650", Make: "Kawasaki", Model: "Ninja 650", Aliases: []string{"Kawasaki Ninja650", "Kawasaki Ninja 650 2017", "Кавасаки Ниндзя 650", "Кавасаки Ninja650"}, Images: []BikeModelImage{{URL: "/bike-models/kawasaki-ninja-650-2017.jpg", YearFrom: 2017, YearTo: 2017, Width: 960, Height: 540, SourceURL: "https://commons.wikimedia.org/wiki/File:Kawasaki_Ninja_650_2017.jpg", Author: "Sebastian6186", License: "CC BY-SA 3.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/3.0/"}}},
	{ID: "suzuki-v-strom-650", Make: "Suzuki", Model: "V-Strom 650", Aliases: []string{"Suzuki DL650 V-Strom", "Suzuki DL 650", "Suzuki VStrom650", "Сузуки В-Стром 650", "Сузуки ДЛ650"}, Images: []BikeModelImage{{URL: "/bike-models/suzuki-v-strom-650-2004.jpg", YearFrom: 2004, YearTo: 2004, Width: 960, Height: 723, SourceURL: "https://commons.wikimedia.org/wiki/File:V-strom650.jpg", Author: "Corsair88", License: "Public domain (author release)", LicenseURL: "https://commons.wikimedia.org/wiki/File:V-strom650.jpg"}, {URL: "/bike-models/suzuki-v-strom-650-2019.jpg", YearFrom: 2019, YearTo: 2019, Width: 960, Height: 719, SourceURL: "https://commons.wikimedia.org/wiki/File:V-Strom_650.jpg", Author: "Benkorashii", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "bmw-r1200gs", Make: "BMW", Model: "R 1200 GS", Aliases: []string{"BMW R1200GS", "BMW R 1200 GS", "БМВ R1200GS", "БМВ Р 1200 ГС"}, Images: []BikeModelImage{{URL: "/bike-models/bmw-r1200gs-2004.jpg", YearFrom: 2004, YearTo: 2004, Width: 960, Height: 720, SourceURL: "https://commons.wikimedia.org/wiki/File:R1200GS_2004.jpg", Author: "User:Gsmonkey", License: "CC BY 3.0", LicenseURL: "https://creativecommons.org/licenses/by/3.0/"}}},
	{ID: "bmw-s1000rr", Make: "BMW", Model: "S 1000 RR", Aliases: []string{"BMW S1000RR", "BMW S 1000 RR", "БМВ S1000RR", "БМВ С 1000 РР"}, Images: []BikeModelImage{{URL: "/bike-models/bmw-s1000rr-2022.jpg", YearFrom: 2022, YearTo: 2022, Width: 828, Height: 782, SourceURL: "https://commons.wikimedia.org/wiki/File:2022_BMW_S1000RR.jpg", Author: "Samihasib", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "ktm-1290-super-duke-r", Make: "KTM", Model: "1290 Super Duke R", Aliases: []string{"KTM 1290 Superduke R", "KTM 1290 Super Duke R", "КТМ 1290 Супер Дюк Р", "КТМ 1290 Super Duke R"}, Images: []BikeModelImage{{URL: "/bike-models/ktm-1290-super-duke-r-2014.jpg", YearFrom: 2014, YearTo: 2014, Width: 960, Height: 720, SourceURL: "https://commons.wikimedia.org/wiki/File:KTM_Superduke_1290_02.JPG", Author: "Xabi Rome-Hérault", License: "CC BY 3.0", LicenseURL: "https://creativecommons.org/licenses/by/3.0/"}}},
	{ID: "ktm-390-adventure", Make: "KTM", Model: "390 Adventure", Aliases: []string{"KTM 390 Adventure", "KTM Adventure 390", "КТМ 390 Адвенчер", "КТМ 390 Adventure"}, Images: []BikeModelImage{{URL: "/bike-models/ktm-390-adventure-2024.png", YearFrom: 2024, YearTo: 2024, Width: 918, Height: 629, SourceURL: "https://commons.wikimedia.org/wiki/File:Ktm-390-adventure-2024.png", Author: "KTM AG", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "triumph-street-triple-r", Make: "Triumph", Model: "Street Triple R", Aliases: []string{"Triumph Street Triple R", "Triumph StreetTriple R", "Триумф Стрит Трипл Р", "Триумф Street Triple R"}, Images: []BikeModelImage{{URL: "/bike-models/triumph-street-triple-r-2013.jpg", YearFrom: 2013, YearTo: 2013, Width: 960, Height: 640, SourceURL: "https://commons.wikimedia.org/wiki/File:Triumph_Street_Triple_R_2013_with_Bodis_exhaust.jpg", Author: "FanaticTRX", License: "CC BY-SA 3.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/3.0/"}}},
	{ID: "royal-enfield-interceptor-650", Make: "Royal Enfield", Model: "Interceptor 650", Aliases: []string{"Royal Enfield 650 Interceptor", "Royal Enfield Interceptor 650", "Роял Энфилд Интерцептор 650", "Роял Энфилд Interceptor 650"}, Images: []BikeModelImage{{URL: "/bike-models/royal-enfield-interceptor-650-2019.jpg", YearFrom: 2019, YearTo: 2019, Width: 960, Height: 763, SourceURL: "https://commons.wikimedia.org/wiki/File:Royal_Enfield_650_Interceptor.jpg", Author: "Ofml", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
	{ID: "royal-enfield-himalayan", Make: "Royal Enfield", Model: "Himalayan", Aliases: []string{"Royal Enfield Himalayan", "Роял Энфилд Хималаян", "Роял Энфилд Гималаян", "Роял Энфилд Himalayan"}, Images: []BikeModelImage{{URL: "/bike-models/royal-enfield-himalayan-2020.jpg", YearFrom: 2020, YearTo: 2020, Width: 960, Height: 659, SourceURL: "https://commons.wikimedia.org/wiki/File:Royal_Enfield_Himalayan_DB2K5442.jpg", Author: "Ashley Pomeroy", License: "CC0 1.0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/"}}},
	{ID: "cfmoto-700mt", Make: "CFMOTO", Model: "700MT", Aliases: []string{"CFMOTO 700 MT", "CF Moto 700MT", "СиЭфМото 700 МТ", "СФМото 700MT"}, Images: []BikeModelImage{{URL: "/bike-models/cfmoto-700mt-2024.jpg", YearFrom: 2024, YearTo: 2024, Width: 960, Height: 1194, SourceURL: "https://commons.wikimedia.org/wiki/File:CFMOTO_700_MT_2024.jpg", Author: "Ingeworld", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/"}}},
}

func catalogueKey(value string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(value) {
		if unicode.IsLetter(c) || unicode.IsNumber(c) {
			b.WriteRune(c)
		}
	}
	return b.String()
}
func copyBikeModel(value BikeModel) BikeModel {
	value.Aliases = append([]string{}, value.Aliases...)
	value.Images = append([]BikeModelImage{}, value.Images...)
	return value
}

func FindBikeModel(id string) (BikeModel, bool) {
	for _, entry := range bikeModelVocabulary {
		if entry.ID == id {
			return copyBikeModel(entry), true
		}
	}
	return BikeModel{}, false
}
func MatchBikeModels(query string) []BikeModel {
	key := catalogueKey(query)
	out := []BikeModel{}
	for _, entry := range bikeModelVocabulary {
		match := key == "" || strings.Contains(catalogueKey(entry.Make+entry.Model), key)
		if !match {
			for _, alias := range entry.Aliases {
				if strings.Contains(catalogueKey(alias), key) {
					match = true
					break
				}
			}
		}
		if match {
			out = append(out, copyBikeModel(entry))
		}
	}
	return out
}
