package main

import (
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/grafana/pyroscope-go"
	_ "github.com/mattn/go-sqlite3"
)

type imageButton struct {
	widget.BaseWidget
	onTapped func()
	image    *canvas.Image
}

func newImageButton(resource fyne.Resource, tapped func()) *imageButton {
	img := &imageButton{onTapped: tapped}
	img.ExtendBaseWidget(img)
	img.image = canvas.NewImageFromResource(resource)
	img.image.FillMode = canvas.ImageFillContain
	img.image.SetMinSize(fyne.NewSize(150, 150))
	return img
}

func (b *imageButton) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.image)
}

// func (b *imageButton) Tapped(*fyne.PointEvent) {
// 	if b.onTapped != nil {
// 		b.onTapped()
// 	}
// }

var (
	imageTypes = map[string]struct{}{
		".jpg": {}, ".png": {}, ".jpeg": {}, ".gif": {}, ".bmp": {}, ".ico": {},
	}
	resourceCache sync.Map
	logger        *log.Logger
)

func main() {
	logger = log.New(os.Stdout, "", log.LstdFlags)

	// setupProfiling()
	db := setupDatabase()
	defer db.Close()

	fmt.Println("Add pagination/infinite scroll")
	fmt.Println("Add color to tags")
	fmt.Println("Add time created to db")
	fmt.Println("Add sorting by time created")
	fmt.Println("Minimize widget updates:
Fyne's object tree walking is often triggered by widget updates. Try to reduce unnecessary updates by:

Only updating widgets when their data actually changes
Using Fyne's binding system for automatic updates
Batching updates where possible


Optimize layout:
Complex layouts can lead to more time-consuming tree walks. Consider:

Simplifying your UI structure
Using containers efficiently (e.g., VBox, HBox instead of nested containers)
Avoiding deep nesting of widgets


Use canvas objects:
For static or infrequently changing elements, consider using canvas objects instead of widgets. These are generally more lightweight.
Implement custom widgets:
If you have complex custom widgets, ensure they're implemented efficiently. Override the Refresh() method to minimize unnecessary redraws.
Lazy loading:
For large datasets or complex UIs, implement lazy loading techniques to render only visible elements.
Caching:
Implement caching mechanisms for expensive computations or frequently accessed data.
Background processing:
Move time-consuming operations off the main thread using goroutines, updating the UI only when necessary.
Profiling and benchmarking:
Continue using Go's profiling tools to identify specific bottlenecks. You might want to create benchmarks for critical parts of your app.")

	a := app.New()
	w := setupMainWindow(a)

	content := container.NewVBox()
	scroll := container.NewVScroll(content)

	sidebar := container.NewVBox()
	sidebarScroll := container.NewScroll(sidebar)
	sidebarScroll.Hide()

	split := container.NewHSplit(scroll, sidebarScroll)
	split.Offset = 1 // Start with sidebar hidden

	displayImages := createDisplayImagesFunction(db, w, sidebar, sidebarScroll, split, a)

	testPath := getTestPath()
	content.RemoveAll()
	displayImages(testPath)
	// displayImage(db, w, testPath, content, sidebar, sidebarScroll, split, a)

	input := widget.NewEntry()
	input.SetPlaceHolder("Enter a Tag to Search by")

	form := &widget.Form{
		Items: []*widget.FormItem{{Text: "Tag", Widget: input}},
		OnSubmit: func() {
			tagName := input.Text
			imagePaths, err := searchImagesByTag(db, tagName)
			if err != nil {
				fmt.Print("searchImagesByTag")
				dialog.ShowError(err, w)
				return
			}
			updateContentWithSearchResults(content, imagePaths, db, w, sidebar, sidebarScroll, split, a)
		},
	}

	settingsButton := widget.NewButton("Settings", func() {
		createSettingsWindow(a, w, db)
	})

	controls := container.NewBorder(nil, nil, nil, settingsButton, form)
	mainContainer := container.NewBorder(controls, nil, nil, nil, split)

	// controls := container.NewBorder(nil, nil, nil, nil, form)
	// mainContainer := container.NewBorder(controls, nil, nil, nil, split)

	w.SetContent(mainContainer)
	w.ShowAndRun()
}

func setupProfiling() {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)
	pyroscope.Start(pyroscope.Config{
		ApplicationName: "explorer.golang.app",
		ServerAddress:   "http://localhost:4040",
		Logger:          pyroscope.StandardLogger,
		Tags:            map[string]string{"hostname": os.Getenv("HOSTNAME")},
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
}

func setupDatabase() *sql.DB {
	db, err := sql.Open("sqlite3", "file:./index.db")
	if err != nil {
		logger.Fatal("Failed to open database: ", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		logger.Fatal("Failed to connect to database: ", err)
	}
	logger.Println("Connected to db!")

	setupTables(db)
	return db
}

func setupTables(db *sql.DB) {
	tables := []string{
		"CREATE TABLE IF NOT EXISTS `Tag`(`id` INTEGER PRIMARY KEY NOT NULL, `name` VARCHAR(255) NOT NULL, `color` VARCHAR(7) NOT NULL);",
		"CREATE TABLE IF NOT EXISTS `Image`(`id` INTEGER PRIMARY KEY NOT NULL, `path` VARCHAR(1024) NOT NULL);",
		"CREATE TABLE IF NOT EXISTS `ImageTag`(`imageId` INTEGER NOT NULL, `tagId` INTEGER NOT NULL);",
	}
	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			logger.Fatal("Failed to create table: ", err)
		}
	}
}

func setupMainWindow(a fyne.App) fyne.Window {
	w := a.NewWindow("File Explorer")
	w.Resize(fyne.NewSize(1000, 600))

	icon, err := fyne.LoadResourceFromPath("app.ico")
	if err != nil {
		logger.Fatal("Failed to load icon: ", err)
	}
	a.SetIcon(icon)
	w.SetIcon(icon)

	return w
}

func getTestPath() string {
	if runtime.GOOS == "linux" {
		return "/home/amaterasu/Pictures/"
	}
	return `C:\Users\Silvestrs\Desktop\test`
}

func createDisplayImagesFunction(db *sql.DB, w fyne.Window, sidebar *fyne.Container, sidebarScroll *container.Scroll, split *container.Split, a fyne.App) func(string) {
	return func(dir string) {
		files, err := os.ReadDir(dir)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		imageContainer := container.NewAdaptiveGrid(4)
		loadingIndicator := widget.NewProgressBarInfinite()
		content := container.NewVBox(loadingIndicator, imageContainer)

		var wg sync.WaitGroup
		semaphore := make(chan struct{}, runtime.NumCPU())

		for _, file := range files {
			if !file.IsDir() && isImageFile(file.Name()) {
				imgPath := filepath.Join(dir, file.Name())
				wg.Add(1)
				go func(path string) {
					defer wg.Done()
					semaphore <- struct{}{}
					defer func() { <-semaphore }()

					displayImage(db, w, path, imageContainer, sidebar, sidebarScroll, split, a)
				}(imgPath)
			}
		}

		go func() {
			wg.Wait()
			content.Remove(loadingIndicator)
			canvas.Refresh(content)
		}()
	}
}

func isFile(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return !fileInfo.IsDir(), nil
}

func displayImage(db *sql.DB, w fyne.Window, path string, imageContainer *fyne.Container, sidebar *fyne.Container, sidebarScroll *container.Scroll, split *container.Split, a fyne.App) {
	placeholderResource := fyne.NewStaticResource("placeholder", []byte{})
	imgButton := newImageButton(placeholderResource, nil)

	truncatedName := truncateFilename(filepath.Base(path), 10)
	db.Exec("INSERT OR IGNORE INTO Image (path) VALUES (?)", path)
	label := widget.NewLabel(truncatedName)

	imageTile := container.New(layout.NewVBoxLayout(), imgButton, label)
	imageContainer.Add(imageTile)

	resourceChan := make(chan fyne.Resource, 1)

	// codeium
	// go func() {
	// 	resource, err := loadImageResource(path)
	// 	if err != nil {
	// 		logger.Printf("Error loading image %s: %v", path, err)
	// 		resourceChan <- placeholderResource
	// 		canvas.Refresh(imgButton)
	// 		return
	// 	}

	// 	imgButton.image.Resource = resource
	// 	canvas.Refresh(imgButton)
	// 	resourceChan <- resource
	// }()
	// resource := <-resourceChan

	// claude ai
	go func() {
		resource, err := loadImageResourceEfficient(path)
		if err != nil {
			logger.Printf("Error loading image %s: %v", path, err)
			resourceChan <- placeholderResource
			canvas.Refresh(imgButton)
			return
		}

		imgButton.image.Resource = resource
		canvas.Refresh(imgButton)
		resourceChan <- resource
	}()

	// t1 := <-resourceChan

	fmt.Println("Displaying image:", path)

	imgButton.onTapped = func() {
		resource := <-resourceChan
		updateSidebar(db, w, path, resource, sidebar, sidebarScroll, split, a)
		// updateSidebar(db, w, path, t1, sidebar, sidebarScroll, split, a)
	}

	// w.Canvas().Content().Refresh()
}

func loadImageResource(path string) (fyne.Resource, error) {
	if cachedResource, ok := resourceCache.Load(path); ok {
		return cachedResource.(fyne.Resource), nil
	}

	resource, err := fyne.LoadResourceFromPath(path)
	if err != nil {
		return nil, err
	}

	resourceCache.Store(path, resource)
	return resource, nil
}

func updateSidebar(db *sql.DB, w fyne.Window, path string, resource fyne.Resource, sidebar *fyne.Container, sidebarScroll *container.Scroll, split *container.Split, a fyne.App) {
	sidebar.RemoveAll()

	fullImg := canvas.NewImageFromResource(resource)
	fullImg.FillMode = canvas.ImageFillContain
	fullImg.SetMinSize(fyne.NewSize(200, 200))

	fullLabel := widget.NewLabel(filepath.Base(path))
	fullLabel.Wrapping = fyne.TextWrapWord

	imageId := getImageId(db, path)
	tagDisplay := createTagDisplay(db, imageId)

	addTagButton := widget.NewButton("+", func() {
		showTagWindow(a, w, db, imageId, tagDisplay)
	})

	createTagButton := widget.NewButton("Add Tag", func() {
		createTagWindow(a, w, db)
	})

	sidebar.Add(fullImg)
	sidebar.Add(fullLabel)
	sidebar.Add(tagDisplay)
	sidebar.Add(addTagButton)
	sidebar.Add(createTagButton)

	sidebarScroll.Show()
	split.Offset = 0.6
	sidebar.Refresh()
}

func getImageId(db *sql.DB, path string) int {
	var imageId int
	err := db.QueryRow("SELECT id FROM Image WHERE path = ?", path).Scan(&imageId)
	if err != nil {
		logger.Println("Error getting image ID:", err)
		return 0
	}
	return imageId
}

func showTagWindow(a fyne.App, parent fyne.Window, db *sql.DB, imgId int, tagList *fyne.Container) {
	tagWindow := a.NewWindow("Tags")
	tagWindow.SetTitle("Add a Tag")

	content := container.NewGridWithColumns(4)
	loadingLabel := widget.NewLabel("Loading tags...")
	content.Add(loadingLabel)

	tagWindow.SetContent(container.NewVScroll(content))
	tagWindow.Resize(fyne.NewSize(300, 200))
	tagWindow.Show()

	go func() {
		tags, err := db.Query("SELECT id, name FROM Tag WHERE id NOT IN (SELECT tagId FROM ImageTag WHERE imageId = ?)", imgId)
		if err != nil {
			parent.Canvas().Refresh(parent.Content())
			fmt.Print("showTagWindow")
			dialog.ShowError(err, parent)
			return
		}
		defer tags.Close()

		var buttons []*widget.Button
		for tags.Next() {
			var id int
			var name string
			if err := tags.Scan(&id, &name); err != nil {
				parent.Canvas().Refresh(parent.Content())
				fmt.Print("showTagWindow")
				dialog.ShowError(err, parent)
				return
			}

			button := widget.NewButton(name, nil)
			tagID := id
			button.OnTapped = func() {
				go func() {
					_, err := db.Exec("INSERT OR IGNORE INTO ImageTag (imageId, tagId) VALUES (?, ?)", imgId, tagID)
					parent.Content().Refresh()
					if err != nil {
						fmt.Print("showTagWindow")
						dialog.ShowError(err, parent)
					} else {
						tagList.Add(widget.NewButton(name, nil))
						dialog.ShowInformation("Success", "Tag Added", parent)
						tagWindow.Close()
					}
				}()
			}
			buttons = append(buttons, button)
		}

		tagWindow.Canvas().Refresh(content)
		content.Remove(loadingLabel)
		for _, button := range buttons {
			content.Add(button)
		}
	}()
}

func createTagWindow(a fyne.App, parent fyne.Window, db *sql.DB) {
	tagWindow := a.NewWindow("Create Tag")
	tagWindow.SetTitle("Create a Tag")

	var chosenColor color.Color = color.White
	colorButton := widget.NewButton("Choose Tag Color", nil)
	stringInput := widget.NewEntry()
	stringInput.SetPlaceHolder("Enter Tag name")

	updateColorButton := func(c color.Color) {
		chosenColor = c
		r, g, b, _ := c.RGBA()
		colorButton.SetText(fmt.Sprintf("Color: #%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8)))
	}

	colorButton.OnTapped = func() {
		dialog.ShowColorPicker("Choose Tag Color", "Select a color for your tag", updateColorButton, tagWindow)
	}

	createButton := widget.NewButton("Create Tag", func() {
		tagName := stringInput.Text
		if tagName == "" {
			dialog.ShowInformation("Error", "Tag name cannot be empty", tagWindow)
			return
		}
		r, g, b, _ := chosenColor.RGBA()
		hexColor := fmt.Sprintf("#%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8))
		_, err := db.Exec("INSERT INTO Tag (name, color) VALUES (?, ?)", tagName, hexColor)
		if err != nil {
			fmt.Print("createTagWindow")
			dialog.ShowError(err, tagWindow)
			return
		}
		dialog.ShowInformation("Tag Created", "Tag Name: "+tagName+"\nColor: "+hexColor, tagWindow)
		// tagWindow.Close()
	})

	content := container.NewVBox(
		widget.NewLabel("Choose tag color:"),
		colorButton,
		widget.NewLabel("Enter tag name:"),
		stringInput,
		createButton,
	)

	tagWindow.SetContent(content)
	tagWindow.Resize(fyne.NewSize(300, 200))
	tagWindow.Show()
}

func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := imageTypes[ext]
	return ok
}

func truncateFilename(filename string, maxLength int) string {
	// get the file extension
	ext := filepath.Ext(filename)
	// get the filename without extension
	nameWithoutExt := filename[:len(filename)-len(ext)]
	// if filename without extension is bigger or equal to maxLength, return filename with extension
	if len(nameWithoutExt) <= maxLength {
		return filename
	} else {
		return nameWithoutExt[:maxLength] + ext
	}
}

// New function to handle image loading more efficiently
// func loadImagesAsync(dir string, imageContainer *fyne.Container, db *sql.DB, w fyne.Window, sidebar *fyne.Container, sidebarScroll *container.Scroll, split *container.Split, a fyne.App) {
// 	files, err := os.ReadDir(dir)
// 	if err != nil {
// 		fmt.Print("loadImagesAsync")
// 		dialog.ShowError(err, w)
// 		return
// 	}

// 	var wg sync.WaitGroup
// 	semaphore := make(chan struct{}, runtime.NumCPU()) // Limit concurrent goroutines

// 	for _, file := range files {
// 		if file.IsDir() || !isImageFile(file.Name()) {
// 			continue
// 		}

// 		wg.Add(1)
// 		go func(f os.DirEntry) {
// 			defer wg.Done()
// 			semaphore <- struct{}{}
// 			defer func() { <-semaphore }()

// 			imgPath := filepath.Join(dir, f.Name())
// 			displayImage(db, w, imgPath, imageContainer, sidebar, sidebarScroll, split, a)
// 		}(file)
// 	}

// 	wg.Wait()
// 	imageContainer.Refresh()
// }

// Optimized function to load image resources
func loadImageResourceEfficient(path string) (fyne.Resource, error) {
	if cachedResource, ok := resourceCache.Load(path); ok {
		return cachedResource.(fyne.Resource), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Create a new static resource
	resource := fyne.NewStaticResource(filepath.Base(path), content)

	// Store in cache
	resourceCache.Store(path, resource)

	return resource, nil
}

// Function to handle tag-based search
func searchImagesByTag(db *sql.DB, tagName string) ([]string, error) {
	query := `
		SELECT DISTINCT Image.path
		FROM Image
		JOIN ImageTag ON Image.id = ImageTag.imageId
		JOIN Tag ON ImageTag.tagId = Tag.id
		WHERE Tag.name = ?
	`
	rows, err := db.Query(query, tagName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var imagePaths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		imagePaths = append(imagePaths, path)
	}

	return imagePaths, nil
}

// Function to update the main content based on search results
func updateContentWithSearchResults(content *fyne.Container, imagePaths []string, db *sql.DB, w fyne.Window, sidebar *fyne.Container, sidebarScroll *container.Scroll, split *container.Split, a fyne.App) {
	content.RemoveAll()
	imageContainer := container.NewAdaptiveGrid(4)
	content.Add(imageContainer)

	for _, path := range imagePaths {
		displayImage(db, w, path, imageContainer, sidebar, sidebarScroll, split, a)
	}

	content.Refresh()
}

// Add a function to remove a tag from an image
func removeTagFromImage(db *sql.DB, imageId int, tagId int) error {
	_, err := db.Exec("DELETE FROM ImageTag WHERE imageId = ? AND tagId = ?", imageId, tagId)
	return err
}

// Modify the createTagDisplay function to include tag removal functionality
func createTagDisplay(db *sql.DB, imageId int) *fyne.Container {
	tagDisplay := container.NewAdaptiveGrid(4)

	rows, err := db.Query("SELECT Tag.id, Tag.name, Tag.color FROM ImageTag INNER JOIN Tag ON ImageTag.tagId = Tag.id WHERE ImageTag.imageId = ?", imageId)
	if err != nil {
		logger.Println("Error querying image tags:", err)
		return tagDisplay
	}
	defer rows.Close()

	for rows.Next() {
		var tagId int
		var tagName, tagColor string
		if err := rows.Scan(&tagId, &tagName, &tagColor); err != nil {
			logger.Println("Error scanning tag data:", err)
			continue
		}

		tagButton := widget.NewButton(tagName, nil)
		tagButton.Importance = widget.LowImportance

		// Set button color based on the tag color
		if c, err := colorFromHex(tagColor); err == nil {
			// rect := canvas.NewRectangle(c)
			fmt.Print(c)
			img := image.NewRGBA(image.Rect(0, 0, 100, 100))
			test := fyne.NewStaticResource("tag", img.Pix)
			// draw.Draw(img, img.Bounds(), &image.Uniform{rect.FillColor}, image.Point{}, draw.Src)

			tagButton.SetIcon(test)
		}

		tagButton.OnTapped = func() {
			dialog.ShowConfirm("Remove Tag", "Are you sure you want to remove this tag?", func(remove bool) {
				if remove {
					if err := removeTagFromImage(db, imageId, tagId); err != nil {
						fmt.Print("createTagDisplay")
						dialog.ShowError(err, nil)
					} else {
						// remove the tag from the tag display
						// refreshSidebar()
					}
				}
			}, nil)
		}

		tagDisplay.Add(tagButton)
	}

	return tagDisplay
}

// func createTagDisplay(db *sql.DB, imageId int) *fyne.Container {
// 	tagDisplay := container.NewAdaptiveGrid(4)

// 	rows, err := db.Query("SELECT Tag.name FROM ImageTag INNER JOIN Tag ON ImageTag.tagId = Tag.id WHERE ImageTag.imageId = ?", imageId)
// 	if err != nil {
// 		logger.Println("Error querying image tags:", err)
// 		return tagDisplay
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var tagName string
// 		if err := rows.Scan(&tagName); err != nil {
// 			logger.Println("Error scanning tag name:", err)
// 			continue
// 		}
// 		tagDisplay.Add(widget.NewButton(tagName, nil))
// 	}

// 	return tagDisplay
// }

// Helper function to convert hex color to color.Color
func colorFromHex(hex string) (color.Color, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return nil, fmt.Errorf("invalid hex color")
	}
	rgb, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return nil, err
	}
	return color.RGBA{
		R: uint8(rgb >> 16),
		G: uint8(rgb >> 8 & 0xFF),
		B: uint8(rgb & 0xFF),
		A: 255,
	}, nil
}

// Add a settings window
func createSettingsWindow(a fyne.App, parent fyne.Window, db *sql.DB) {
	settingsWindow := a.NewWindow("Settings")

	// Create a form for database path
	dbPathEntry := widget.NewEntry()
	dbPathEntry.SetText("./index.db") // Set current path

	dbPathForm := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Database Path", Widget: dbPathEntry},
		},
		OnSubmit: func() {
			// Here you would implement the logic to change the database path
			// This might involve closing the current connection, copying the database, and opening a new connection
			dialog.ShowInformation("Database Path", "Path updated to: "+dbPathEntry.Text, settingsWindow)
		},
	}

	// Create a list of all tags
	tagList := widget.NewList(
		func() int {
			// Return the number of tags
			var count int
			db.QueryRow("SELECT COUNT(*) FROM Tag").Scan(&count)
			return count
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Tag Name")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			label := item.(*widget.Label)
			var tagName string
			db.QueryRow("SELECT name FROM Tag WHERE id = ?", id+1).Scan(&tagName)
			label.SetText(tagName)
		},
	)

	// Create a container for the settings content
	content := container.NewVBox(
		widget.NewLabel("Settings"),
		dbPathForm,
		widget.NewLabel("Tags"),
		tagList,
	)

	settingsWindow.SetContent(content)
	settingsWindow.Resize(fyne.NewSize(400, 300))
	settingsWindow.Show()
}
