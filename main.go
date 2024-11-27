package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Product represents the structure of a product from the first API
type Product struct {
	ID int `json:"id"`
}

// CategoryRes represents the structure of the first API response
type CategoryRes struct {
	Status int `json:"status"`
	Data   struct {
		Products []Product `json:"products"`
	} `json:"data"`
}

// ProductRes represents the structure of the second API response
type ProductRes struct {
	Status int `json:"status"`
	Data   struct {
		Product struct {
			Images struct {
				Main struct {
					URLs []string `json:"url"`
				} `json:"main"`
				List []struct {
					URLs []string `json:"url"`
				} `json:"list"`
			} `json:"images"`
		} `json:"product"`
	} `json:"data"`
}

const (
	baseURL           = "https://api.digikala.com/v1/categories/kids-apparel/search/?th_no_track=1&page=" // Replace with the actual API URL
	productDetailsURL = "https://api.digikala.com/v2/product/"                                            // Replace with the actual product API URL
	concurrentLimit   = 1                                                                                 // Number of concurrent requests
)

func main() {
	productChan := make(chan int, concurrentLimit) // Channel to handle product IDs
	var wg sync.WaitGroup                          // WaitGroup to ensure all goroutines complete

	// Launch workers to fetch product details and download images
	for i := 0; i < concurrentLimit; i++ {
		wg.Add(1)
		go productWorker(productChan, &wg)
	}

	// Fetch products for each page
	for page := 1; page <= 100; page++ {
		url := baseURL + strconv.Itoa(page)
		fmt.Printf("Fetching page: %d\n", page)

		products, err := fetchProducts(url)
		if err != nil {
			fmt.Printf("Failed to fetch page %d: %v\n", page, err)
			continue
		}

		for _, product := range products {
			productChan <- product.ID
		}
	}

	close(productChan) // Close the channel after feeding all product IDs
	wg.Wait()          // Wait for all workers to finish
	fmt.Println("All tasks completed.")
}

// fetchProducts fetches products from a given page URL
func fetchProducts(url string) ([]Product, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	var response CategoryRes
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Data.Products, nil
}

// fetchProductDetails fetches product details including image URLs
// fetchProductDetails fetches product details including all image URLs
func fetchProductDetails(productID int) ([]string, error) {
	url := productDetailsURL + strconv.Itoa(productID) + "/"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch product %d details: %w", productID, err)
	}
	defer resp.Body.Close()

	var response ProductRes
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode product %d details: %w", productID, err)
	}

	// Collect all image URLs
	var imageURLs []string
	imageURLs = append(imageURLs, response.Data.Product.Images.Main.URLs...) // Add main URLs

	for _, item := range response.Data.Product.Images.List {
		imageURLs = append(imageURLs, item.URLs...) // Add list URLs
	}

	return imageURLs, nil
}

// downloadImage downloads the image from the given URL and saves it locally
func downloadImage(url, filename string) error {
	// Create the ./img directory if it doesn't exist
	imageDir := "./img"
	if err := os.MkdirAll(imageDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Construct the full file path
	filePath := filepath.Join(imageDir, filename)

	// Fetch the image
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	// Create the file in the specified directory
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy the response body to the file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	fmt.Printf("Image saved as %s\n", filePath)
	return nil
}

// productWorker handles fetching product details and downloading images concurrently
func productWorker(productChan <-chan int, wg *sync.WaitGroup) {
	defer wg.Done()

	for productID := range productChan {
		fmt.Printf("Fetching details for product ID: %d\n", productID)
		imageURLs, err := fetchProductDetails(productID)
		if err != nil {
			fmt.Printf("Failed to fetch product %d details: %v\n", productID, err)
			continue
		}

		for i, imgURL := range imageURLs {
			filename := fmt.Sprintf("product_%d_img_%d.jpg", productID, i+1)
			if err := downloadImage(imgURL, filename); err != nil {
				fmt.Printf("Failed to download image for product %d: %v\n", productID, err)
			}
		}
	}
}
