package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func main() {
	launchURL := launcher.New().Headless(true).MustLaunch()
	browser := rod.New().ControlURL(launchURL).MustConnect()
	defer browser.Close()

	fmt.Println("Start...")
	page := browser.MustPage("https://www.hama-sushi.co.jp/menu/")
	page.MustWaitLoad()

	// スクロールによる要素の読み込みを繰り返す
	fmt.Println("Scrolling to load all items...")
	scrollStep := 500
	maxScrollPx := 25000
	currentScroll := 0
	for {
		if currentScroll >= maxScrollPx {
			fmt.Println("Reached max scroll.", maxScrollPx)
			break
		}

		fmt.Println("Scrolling...: ", currentScroll)
		page.MustEval(fmt.Sprintf(`() => window.scrollBy(0, %d)`, scrollStep))
		currentScroll += scrollStep
		time.Sleep(1 * time.Second)
	}

	// スクロールしたところまでのloadingされたimgを取得
	fmt.Println("Getting image elements...")
	items, err := page.Elements(`div.men-products-item__thumb img:not([src*="loading.gif"])`)
	if err != nil {
		panic(err)
	}

	downloadDir := "downloads"
	info, err := page.Info()
	if err != nil {
		panic(err)
	}
	baseURL, _ := url.Parse(info.URL)

	for _, item := range items {
		srcAttr, err := item.Attribute("src")
		if err != nil || srcAttr == nil || *srcAttr == "" {
			continue
		}

		// 相対URLの場合があるので絶対URLへ
		relURL, err := url.Parse(*srcAttr)
		if err != nil {
			fmt.Printf("invalid src url: %v\n", err)
			continue
		}
		imgURL := baseURL.ResolveReference(relURL).String()

		dirPath, fileName := path.Split(relURL.Path)              // "/assets/menu/img/dessert/", "pho_sake.png"
		folderName := path.Base(strings.TrimSuffix(dirPath, "/")) // "/assets/menu/img/dessert/pho_sake.png", "dessert"
		subDir := filepath.Join(downloadDir, folderName)
		// フォルダが存在しない場合は作成
		if err := os.MkdirAll(subDir, 0755); err != nil {
			fmt.Printf("failed to create dir %s: %v\n", subDir, err)
			continue
		}

		localPath := filepath.Join(subDir, fileName)
		if err := downloadImage(context.Background(), imgURL, localPath); err != nil {
			fmt.Printf("failed to download %s: %v\n", imgURL, err)
		} else {
			fmt.Printf("downloaded: %s -> %s\n", imgURL, localPath)
		}
	}

	fmt.Println("Done.")
}

func downloadImage(ctx context.Context, url, filepath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200 status code: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
