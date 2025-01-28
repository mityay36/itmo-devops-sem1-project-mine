package handlers

import (
	"archive/tar"
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
    "fmt"
	"project_sem/db"
)

type Response struct {
	TotalItems     int     `json:"total_items"`
	TotalCategories int    `json:"total_categories"`
	TotalPrice     float64 `json:"total_price"`
}

// POST /api/v0/prices
func UploadPrices(w http.ResponseWriter, r *http.Request) {
	// Получение типа архива
	archiveType := r.URL.Query().Get("type")
	if archiveType == "" {
		archiveType = "zip"
	}

	// Чтение файла из тела запроса
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка при чтении файла: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Сохранение архива
	tempFile := "temp." + archiveType
	temp, err := os.Create(tempFile)
	if err != nil {
		http.Error(w, "Ошибка при создании временного файла: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile)
	defer temp.Close()

    log.Println("Начало копирования файла")
    n, err := io.Copy(temp, file)
    if err != nil {
        log.Println("Ошибка при копировании:", err)
        http.Error(w, "Ошибка при копировании файла: "+err.Error(), http.StatusInternalServerError)
        return
    }
    log.Printf("Копирование завершено, %d байт\n", n)


	// Разархивация и запись данных
	items, categories, totalPrice := 0, make(map[string]bool), 0.0
	switch archiveType {
	case "zip":
		items, categories, totalPrice, err = processZip(tempFile)
	case "tar":
		items, categories, totalPrice, err = processTar(tempFile)
	default:
		http.Error(w, "Неподдерживаемый тип архива", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "Ошибка обработки архива: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := Response{
		TotalItems:     items,
		TotalCategories: len(categories),
		TotalPrice:     totalPrice,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func processZip(filePath string) (int, map[string]bool, float64, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return 0, nil, 0, err
    }
    defer file.Close()

    fileInfo, err := file.Stat()
    if err != nil {
        return 0, nil, 0, err
    }

    zipReader, err := zip.NewReader(file, fileInfo.Size())
    if err != nil {
        return 0, nil, 0, err
    }

    var i int
    return processArchive(func() (io.Reader, string, error) {
        for ; i < len(zipReader.File); i++ {
            header := zipReader.File[i]
            if strings.HasSuffix(strings.ToLower(header.Name), ".csv") {
                rc, err := header.Open()
                if err != nil {
                    return nil, "", err
                }
                i++
                return rc, header.Name, nil
            }
        }
        return nil, "", io.EOF
    })
}

func processTar(filePath string) (int, map[string]bool, float64, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return 0, nil, 0, err
    }
    defer file.Close()

    tarReader := tar.NewReader(file)

    var foundEOF bool

    return processArchive(func() (io.Reader, string, error) {
        if foundEOF {
            return nil, "", io.EOF
        }

        for {
            header, err := tarReader.Next()
            if err == io.EOF {
                foundEOF = true
                return nil, "", io.EOF
            }
            if err != nil {
                return nil, "", err
            }

            if strings.HasSuffix(strings.ToLower(header.Name), ".csv") {
                return tarReader, header.Name, nil
            }
        }
    })
}

func processArchive(nextFile func() (io.Reader, string, error)) (int, map[string]bool, float64, error) {
	items, totalPrice := 0, 0.0
	categories := make(map[string]bool)

	// Начало транзакции
	tx, err := db.DB.Begin()
	if err != nil {
		return 0, nil, 0, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			log.Printf("Паника: %v, откат транзакции", p)
			tx.Rollback()
			panic(p)
		} else if err != nil {
			log.Printf("Ошибка: %v, откат транзакции", err)
			tx.Rollback()
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				log.Printf("Ошибка коммита транзакции: %v", commitErr)
				err = commitErr
			}
		}
	}()

	for {
		reader, name, err := nextFile()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, nil, 0, fmt.Errorf("ошибка получения следующего файла: %w", err)
		}

		if filepath.Ext(name) != ".csv" {
			continue
		}

		csvReader := csv.NewReader(reader)
		_, err = csvReader.Read() // Пропускаем заголовок
		if err != nil {
			log.Printf("Ошибка чтения заголовка CSV: %v", err)
			continue
		}

		for {
			record, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("Ошибка чтения строки CSV: %v", err)
				continue
			}

			if len(record) < 5 {
				log.Printf("Некорректная запись в CSV: %v", record)
				continue
			}

			price, parseErr := strconv.ParseFloat(record[3], 64)
			if parseErr != nil {
				log.Printf("Ошибка преобразования цены: %v", parseErr)
				continue
			}

			categories[record[2]] = true
			totalPrice += price
			items++

			_, execErr := tx.Exec(
				"INSERT INTO prices (name, category, price, create_date) VALUES ($1, $2, $3, $4)",
				record[1], record[2], price, record[4],
			)
			if execErr != nil {
				return 0, nil, 0, fmt.Errorf("ошибка записи в базу данных: %w", execErr)
			}
		}
	}
	return items, categories, totalPrice, nil
}


// GET /api/v0/prices
func GetPrices(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT id, name, category, price, create_date FROM prices")
	if err != nil {
		http.Error(w, "Ошибка при чтении из базы данных: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Создание CSV
	tempFile := "data.csv"
	csvFile, err := os.Create(tempFile)
	if err != nil {
		http.Error(w, "Ошибка при создании CSV-файла: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile)
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	writer.Write([]string{"id", "name", "category", "price", "create_date"})
	for rows.Next() {
		var id int
		var name, category, createDate string
		var price float64
		rows.Scan(&id, &name, &category, &price, &createDate)
		writer.Write([]string{strconv.Itoa(id), name, category, strconv.FormatFloat(price, 'f', 2, 64), createDate})
	}
	writer.Flush()

	zipFile, err := os.CreateTemp("", "data-*.zip")
	if err != nil {
		http.Error(w, "Ошибка создания ZIP-файла", http.StatusInternalServerError)
		log.Printf("Ошибка создания ZIP-файла: %v", err)
		return
	}
	defer os.Remove(zipFile.Name())

	zipWriter := zip.NewWriter(zipFile)
	fileWriter, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Ошибка добавления файла в ZIP", http.StatusInternalServerError)
		log.Printf("Ошибка записи в ZIP: %v", err)
		return
	}

	csvFile.Seek(0, io.SeekStart)
	io.Copy(fileWriter, csvFile)
	zipWriter.Close()

	// Отправка zip-архива
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=data.zip")
	http.ServeFile(w, r, zipFile.Name())
}
