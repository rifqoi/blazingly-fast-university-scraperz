package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocarina/gocsv"
)

var fileResult string

type Universitas struct {
	Nama    string   `csv:"nama" json:"nama"`
	Kode    string   `csv:"kode" json:"kode"`
	Website string   `json:"website"`
	IconURL []string `json:"icon_urls"`
}

func (u Universitas) ToJSON() []byte {
	res, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		panic(err)
	}

	return res
}

type DiktiFinalResult struct {
	Npsn             string  `json:"npsn"`
	StatSp           string  `json:"stat_sp"`
	NmLemb           string  `json:"nm_lemb"`
	TglBerdiri       string  `json:"tgl_berdiri"`
	SkPendirianSp    string  `json:"sk_pendirian_sp"`
	TglSkPendirianSp string  `json:"tgl_sk_pendirian_sp"`
	Jln              string  `json:"jln"`
	NamaWil          string  `json:"nama_wil"`
	KodePos          string  `json:"kode_pos"`
	NoTel            string  `json:"no_tel"`
	NoFax            string  `json:"no_fax"`
	Email            string  `json:"email"`
	Website          string  `json:"website"`
	Lintang          float64 `json:"lintang"`
	Bujur            float64 `json:"bujur"`
	IDSp             string  `json:"id_sp"`
	LuasTanah        int     `json:"luas_tanah"`
	Laboratorium     int     `json:"laboratorium"`
	RuangKelas       int     `json:"ruang_kelas"`
	Perpustakaan     int     `json:"perpustakaan"`
	Internet         bool    `json:"internet"`
	Listrik          bool    `json:"listrik"`
	NamaRektor       string  `json:"nama_rektor"`
	AkreditasiList   []struct {
		Akreditasi    string    `json:"akreditasi"`
		TglAkreditasi time.Time `json:"tgl_akreditasi"`
		TglBerlaku    time.Time `json:"tgl_berlaku"`
	} `json:"akreditasi_list"`
}

type DiktiInitialHit struct {
	Dosen []struct {
		Text        string `json:"text"`
		WebsiteLink string `json:"website-link"`
	} `json:"dosen"`
	Prodi []struct {
		Text        string `json:"text"`
		WebsiteLink string `json:"website-link"`
	} `json:"prodi"`
	Pt []struct {
		Text        string `json:"text"`
		WebsiteLink string `json:"website-link"`
	} `json:"pt"`
}

func getJSON(url string, data any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		return err
	}

	return nil
}

func getDiktiUnivDetail(diktiKode string) (*DiktiFinalResult, error) {
	url := fmt.Sprintf("https://api-frontend.kemdikbud.go.id/v2/detail_pt/%s", diktiKode)

	var univDetails *DiktiFinalResult

	err := getJSON(url, &univDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to getDiktiUnivDetail: %v", err)
	}

	return univDetails, nil

}

func getDiktiURL(univKode string, univName string) (string, error) {
	url := fmt.Sprintf("https://api-frontend.kemdikbud.go.id/hit/%s", univKode)
	var hits DiktiInitialHit

	err := getJSON(url, &hits)

	if err != nil {
		return "", fmt.Errorf("Failed to getDiktiURL: %v", err)
	}

	if len(hits.Pt) < 1 {
		return "", fmt.Errorf(univName, "not found!")
	}

	return strings.Replace(hits.Pt[0].WebsiteLink, "/data_pt/", "", -1), nil
}

func readCSV[V any](path string) []V {
	var listUniv []V

	in, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	err = gocsv.UnmarshalFile(in, &listUniv)
	if err != nil {
		log.Fatalf("Cannot unmarshal file %s ", path)
	}

	return listUniv
}

func appendJsonToFile(jsn any, path string) error {

	b, err := json.Marshal(jsn)
	if err != nil {
		return fmt.Errorf("appendJsonToFile error: %v", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("appendJsonToFile error: %v", err)
	}
	defer f.Close()

	_, err = f.Write(b)
	if err != nil {
		return fmt.Errorf("appendJsonToFile error: %v", err)
	}

	if _, err = f.WriteString("\n"); err != nil {
		return fmt.Errorf("appendJsonToFile error: %v", err)
	}

	return nil
}

func scrapePdDikti() {
	listUniv := readCSV[Universitas]("./daftar_pt.csv")

	now := time.Now().Unix()
	jsonResult := fmt.Sprintf("./data-%d.json", now)
	scrape := false
	for i, univ := range listUniv {

		// if univ.Kode == "091007" {
		// 	scrape = true
		// }

		if !scrape {
			continue
		}

		if i%100 == 0 {
			time.Sleep(10)
		}

		diktiCode, err := getDiktiURL(univ.Kode, univ.Nama)
		slog.Info(fmt.Sprintf("Visiting %s", univ.Nama), "univ_code", univ.Kode, "dikti_code", diktiCode)

		if diktiCode == "" {
			slog.Warn(fmt.Sprintf("Dikti Code %s is empty", univ.Nama))
			continue
		}

		if err != nil {
			slog.Error(err.Error())
		}

		univDetail, err := getDiktiUnivDetail(diktiCode)
		slog.Info(fmt.Sprintf("%s Website: %s", univ.Nama, univDetail.Website))
		if err != nil {
			slog.Error(err.Error())
		}

		appendJsonToFile(univDetail, jsonResult)

	}

}

func main() {

	fileResult = "univResult.json"

	in, _ := os.Open("./result.json")

	var diktiResults []DiktiFinalResult
	dec := json.NewDecoder(in)

	for {
		var diktiRes DiktiFinalResult

		err := dec.Decode(&diktiRes)
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		diktiResults = append(diktiResults, diktiRes)

	}

	var wg sync.WaitGroup

	univs := make(chan Universitas, 100)

	go crawlWebs(univs, diktiResults)
	for w := 1; w <= 10; w++ {
		wg.Add(1)

		w := w
		go func() {
			defer wg.Done()
			worker(w, univs)
		}()
	}

	wg.Wait()
}

func worker(wID int, jobs <-chan Universitas) {
	for j := range jobs {
		scrape(j, wID)
	}
}

func scrape(univ Universitas, wID int) {

	slog.Info(fmt.Sprintf("Scraped website: %s", univ.Website), "univ", univ.Nama, "worker", wID)

	ok := strings.HasPrefix(univ.Website, "http")
	if !ok {
		univ.Website = fmt.Sprintf("https://%s", strings.TrimSpace(univ.Website))
	}

	resp, err := http.Get(univ.Website)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("head").Find("link").Each(func(i int, s *goquery.Selection) {
		rel, exists := s.Attr("rel")
		if !exists {
			return
		}

		if strings.TrimSpace(rel) == "icon" {
			if href, exists := s.Attr("href"); exists {
				univ.IconURL = append(univ.IconURL, href)
			}
		}
	})

	appendJsonToFile(univ, fileResult)
}
func crawlWebs(webs chan<- Universitas, items []DiktiFinalResult) {
	for {
		for _, item := range items {
			url := item.Website

			univ := Universitas{}
			univ.Website = url
			univ.Nama = item.NmLemb
			univ.Kode = item.Npsn

			if strings.TrimSpace(url) == "" {
				slog.Warn("URL is empty", "univ", univ.Nama)
				continue
			}

			select {
			case webs <- univ:
				slog.Info(fmt.Sprintf("Scraping website icon: %s", univ.Website))
			default:
				slog.Info("Channel is full. Waiting the channel to be available... ")

				buffer := make([]Universitas, 0, 10)
				buffer = append(buffer, univ)

				for len(buffer) > 0 {
					select {
					case webs <- buffer[0]:
						buffer = buffer[1:]
					default:
						slog.Info("Channel is still full. Waiting....")
						time.Sleep(time.Second)
					}
				}
			}
		}

		break
	}
}
