package utils

import (
	"encoding/json"
	"path/filepath"
	"math/rand"
	"strings"
	"sync"
	"log"
	"os"

	http "github.com/bogdanfinn/fhttp"
)

type CookiesPool struct {
    Pool map[string][]*http.Cookie
}

type JSONCookie struct {
    Name  string `json:"Name"`
    Value string `json:"Value"`
}

var (
	cookiesOnce sync.Once
	instance    *CookiesPool
)

func BuildCookiesPool() *CookiesPool {
	cookiesOnce.Do(func() {
		instance = &CookiesPool{}
		instance.Build("data/cookies")
	})
	return instance
}

func (cp *CookiesPool) GetPoolUsernames() *[]string {
    keys := make([]string, 0, len(cp.Pool))
    for k := range cp.Pool {
        keys = append(keys, k)
    }
    return &keys
}

func (cp *CookiesPool) AddCookiesToPool(username string, cookies *[]*http.Cookie) {
    cp.Pool[username] = *cookies
}

// TODO: sync up with selenium driver
func (cp *CookiesPool) GetRandomCookies() *[]*http.Cookie {
    if len(cp.Pool) == 0 {
        log.Fatalf("No cookies available in CookiesPool. Exiting...")
    }
    keys := *cp.GetPoolUsernames()
    randomKey := keys[rand.Intn(len(keys))]
    randomCookies := cp.Pool[randomKey]
    return &randomCookies
}

func (cp *CookiesPool) GetCookiesPool() *map[string][]*http.Cookie {
    return &cp.Pool
}

func (cp *CookiesPool) RemoveCookiesFromPool(username string) {
    delete(cp.Pool, username)
}
func (cp *CookiesPool) MoveToSuspended(filename string) {
    srcPath := filepath.Join("data/cookies", filename)
    destPath := filepath.Join("data/cookies/suspended", filename)
    if _, err := os.Stat(srcPath); os.IsNotExist(err) {
        log.Printf("File %s does not exist", srcPath)
        return
    }
    if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
        log.Fatalf("Failed to create suspended directory: %v", err)
    }
    if err := os.Rename(srcPath, destPath); err != nil {
        log.Fatalf("Failed to move file %s to suspended: %v", srcPath, err)
    }
}

func (cp *CookiesPool) Build(folderPath string) *map[string][]*http.Cookie {
    if len(cp.Pool) != 0 {
        return &cp.Pool
    }
    cookiesPool := make(map[string][]*http.Cookie)

    entries, err := os.ReadDir(folderPath)
    if err != nil {
        log.Fatalf("Failed to read cookies folder: %v", err)
    }

    for _, entry := range entries {
        if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
            continue
        }
        filePath := filepath.Join(folderPath, entry.Name())
        username := strings.TrimSuffix(entry.Name(), ".json")
        jsonBytes, err := os.ReadFile(filePath)
        if err != nil {
            log.Printf("Failed to read file %s: %v", filePath, err)
            continue
        }
        var jsonCookies []JSONCookie
        if err := json.Unmarshal(jsonBytes, &jsonCookies); err != nil {
            log.Printf("Failed to parse JSON file %s: %v", filePath, err)
            continue
        }
        var httpCookies []*http.Cookie
        for _, c := range jsonCookies {
            httpCookies = append(httpCookies, &http.Cookie{
                Name:  c.Name,
                Value: c.Value,
            })
        }
        cookiesPool[username] = httpCookies
    }

    if len(cookiesPool) == 0 {
        log.Fatalf("No valid cookies loaded from folder: %s", folderPath)
    }

    cp.Pool = cookiesPool
    return &cp.Pool
}


func GetCookieByName(cookies *[]*http.Cookie, name string) *string {
    c := ""
    for _, cookie := range *cookies {
        if cookie.Name == name {
            return &cookie.Value
        }
    }
    return &c
}
