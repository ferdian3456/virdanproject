package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// Log functions with colors
func logInfo(message string) {
	fmt.Printf("%s[INFO]%s %s\n", ColorGreen, ColorReset, message)
}

func logSuccess(message string) {
	fmt.Printf("%s[SUCCESS]%s %s\n", ColorBlue, ColorReset, message)
}

func logWarning(message string) {
	fmt.Printf("%s[WARNING]%s %s\n", ColorYellow, ColorReset, message)
}

func logError(message string) {
	fmt.Printf("%s[ERROR]%s %s\n", ColorRed, ColorReset, message)
}

func logStep(step int, message string) {
	fmt.Printf("%s[STEP %d]%s %s\n", ColorCyan, step, ColorReset, message)
}

// ============== GENERATE FUNCTIONS ==============

// TemplateData holds the data for template generation
type TemplateData struct {
	ModuleName      string
	LowerModuleName string
}

// Templates for each file type
var controllerTemplate = `package http

import (
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type {{.ModuleName}}Controller struct {
	{{.ModuleName}}Usecase *usecase.{{.ModuleName}}Usecase
	Log         *zap.Logger
	Config      *koanf.Koanf
}

func New{{.ModuleName}}Controller({{.LowerModuleName}}Usecase *usecase.{{.ModuleName}}Usecase, zap *zap.Logger, koanf *koanf.Koanf) *{{.ModuleName}}Controller {
	return &{{.ModuleName}}Controller{
		{{.ModuleName}}Usecase: {{.LowerModuleName}}Usecase,
		Log:         zap,
		Config:      koanf,
	}
}
`

var usecaseTemplate = `package usecase

import (
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type {{.ModuleName}}Usecase struct {
	{{.ModuleName}}Repository *repository.{{.ModuleName}}Repository
	DB             *pgxpool.Pool
	Log            *zap.Logger
	Config         *koanf.Koanf
}

func New{{.ModuleName}}Usecase({{.LowerModuleName}}Repository *repository.{{.ModuleName}}Repository, db *pgxpool.Pool, zap *zap.Logger, koanf *koanf.Koanf) *{{.ModuleName}}Usecase {
	return &{{.ModuleName}}Usecase{
		{{.ModuleName}}Repository: {{.LowerModuleName}}Repository,
		DB:             db,
		Log:            zap,
		Config:         koanf,
	}
}
`

var repositoryTemplate = `package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type {{.ModuleName}}Repository struct {
	Log     *zap.Logger
	DB      *pgxpool.Pool
	DBCache *redis.Client
}

func New{{.ModuleName}}Repository(zap *zap.Logger, db *pgxpool.Pool, dbCache *redis.Client) *{{.ModuleName}}Repository {
	return &{{.ModuleName}}Repository{
		Log:     zap,
		DB:      db,
		DBCache: dbCache,
	}
}
`

// App registration template
var appRegistrationTemplate = `	{{.LowerModuleName}}Repository := repository.New{{.ModuleName}}Repository(config.Log, config.DB, config.DBCache)
	{{.LowerModuleName}}Usecase := usecase.New{{.ModuleName}}Usecase({{.LowerModuleName}}Repository, config.DB, config.Log, config.Config)
	{{.LowerModuleName}}Controller := http.New{{.ModuleName}}Controller({{.LowerModuleName}}Usecase, config.Log, config.Config)`

// RouteConfig field template
var routeConfigFieldTemplate = `	{{.ModuleName}}Controller         *http.{{.ModuleName}}Controller`

// RouteConfig initialization template
var routeConfigInitTemplate = `		{{.ModuleName}}Controller:         {{.LowerModuleName}}Controller,`

// Route group template
var routeGroupTemplate = `	{{.LowerModuleName}}Group := api.Group("/{{.LowerModuleName}}", c.AuthMiddleware.ProtectedRoute())
	//{{.LowerModuleName}}Group.Get("", c.{{.ModuleName}}Controller.Get{{.ModuleName}}())
	//{{.LowerModuleName}}Group.Post("", c.{{.ModuleName}}Controller.Create{{.ModuleName}}())
	//{{.LowerModuleName}}Group.Get("/:id", c.{{.ModuleName}}Controller.Get{{.ModuleName}}ById())
	//{{.LowerModuleName}}Group.Patch("/:id", c.{{.ModuleName}}Controller.Update{{.ModuleName}}())
	//{{.LowerModuleName}}Group.Delete("/:id", c.{{.ModuleName}}Controller.Delete{{.ModuleName}}())`

func runGenerate() {
	fmt.Println()
	fmt.Printf("%s%s%s\n", ColorBold, ColorCyan, "=========================================")
	fmt.Printf("%s%s%s\n", ColorBold, ColorCyan, "     Generate Boilerplate")
	fmt.Printf("%s%s%s\n", ColorBold, ColorCyan, "=========================================")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter %sModule Name%s (PascalCase): ", ColorBold, ColorReset)
	moduleName, _ := reader.ReadString('\n')
	moduleName = strings.TrimSpace(moduleName)

	if moduleName == "" {
		logError("Module name is required")
		fmt.Printf("%sExample: %s%s%s\n", ColorYellow, ColorBold, "Product", ColorReset)
		fmt.Printf("%sExample: %s%s%s\n", ColorYellow, ColorBold, "Category", ColorReset)
		fmt.Printf("%sExample: %s%s%s\n", ColorYellow, ColorBold, "Order", ColorReset)
		return
	}

	// Validate module name
	if !isValidModuleName(moduleName) {
		logError(fmt.Sprintf("Invalid module name '%s'. Module name should be in PascalCase.", moduleName))
		fmt.Printf("%sExample: %s%s%s\n", ColorYellow, ColorBold, "Product", ColorReset)
		fmt.Printf("%sExample: %s%s%s\n", ColorYellow, ColorBold, "Category", ColorReset)
		fmt.Printf("%sExample: %s%s%s\n", ColorYellow, ColorBold, "Order", ColorReset)
		return
	}

	data := TemplateData{
		ModuleName:      moduleName,
		LowerModuleName: strings.ToLower(moduleName),
	}

	logStep(1, fmt.Sprintf("Starting generation for module: %s%s%s", ColorBold, moduleName, ColorReset))

	// Generate files
	logStep(2, "Generating controller file...")
	if err := generateFile("internal/delivery/http/"+strings.ToLower(moduleName)+"_controller.go", controllerTemplate, data); err != nil {
		logError(fmt.Sprintf("Failed to generate controller: %v", err))
		return
	}
	logSuccess("Controller file generated successfully")

	logStep(3, "Generating usecase file...")
	if err := generateFile("internal/usecase/"+strings.ToLower(moduleName)+"_usecase.go", usecaseTemplate, data); err != nil {
		logError(fmt.Sprintf("Failed to generate usecase: %v", err))
		return
	}
	logSuccess("Usecase file generated successfully")

	logStep(4, "Generating repository file...")
	if err := generateFile("internal/repository/"+strings.ToLower(moduleName)+"_repository.go", repositoryTemplate, data); err != nil {
		logError(fmt.Sprintf("Failed to generate repository: %v", err))
		return
	}
	logSuccess("Repository file generated successfully")

	logStep(5, "Updating app.go...")
	if err := updateAppGo(data); err != nil {
		logError(fmt.Sprintf("Failed to update app.go: %v", err))
		return
	}
	logSuccess("app.go updated successfully")

	logStep(6, "Updating route.go...")
	if err := updateRouteGo(data); err != nil {
		logError(fmt.Sprintf("Failed to update route.go: %v", err))
		return
	}
	logSuccess("route.go updated successfully")

	logSuccess(fmt.Sprintf("Module %s%s%s generated successfully!", ColorBold, moduleName, ColorReset))
}

func isValidModuleName(name string) bool {
	if len(name) == 0 {
		return false
	}
	// Check if first character is uppercase
	if name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	return true
}

func generateFile(filePath, templateStr string, data TemplateData) error {
	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		logWarning(fmt.Sprintf("File %s already exists", filePath))
		fmt.Printf("Options:\n")
		fmt.Printf("%s1.%s Skip this file\n", ColorYellow, ColorReset)
		fmt.Printf("%s2.%s Overwrite this file\n", ColorRed, ColorReset)
		fmt.Printf("%s3.%s Create backup and overwrite\n", ColorCyan, ColorReset)
		fmt.Printf("Choose option (1/2/3): ")

		var choice string
		_, _ = fmt.Scanln(&choice)

		switch choice {
		case "1":
			logInfo(fmt.Sprintf("Skipping %s", filePath))
			return nil
		case "2":
			logWarning(fmt.Sprintf("Overwriting %s", filePath))
		case "3":
			backupPath := filePath + ".backup"
			if err := copyFile(filePath, backupPath); err != nil {
				return fmt.Errorf("error creating backup: %w", err)
			}
			logSuccess(fmt.Sprintf("Created backup: %s", backupPath))
			logWarning(fmt.Sprintf("Overwriting %s", filePath))
		default:
			logWarning(fmt.Sprintf("Invalid choice. Skipping %s", filePath))
			return nil
		}
	}

	tmpl, err := template.New("file").Parse(templateStr)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	// Format the Go code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		logWarning(fmt.Sprintf("Could not format code for %s: %v", filePath, err))
		formatted = buf.Bytes()
	}

	// Create directory if it doesn't exist
	dir := filePath[:strings.LastIndex(filePath, "/")]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Write file
	// #nosec G306 -- File permissions 0644 are acceptable for source code files
	if err := os.WriteFile(filePath, formatted, 0644); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	logInfo(fmt.Sprintf("Created: %s", filePath))
	return nil
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// #nosec G306 -- File permissions 0644 are acceptable for source code files
	return os.WriteFile(dst, input, 0644)
}

func updateAppGo(data TemplateData) error {
	filePath := "internal/config/app.go"

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading app.go: %w", err)
	}

	// Parse the template
	tmpl, err := template.New("appRegistration").Parse(appRegistrationTemplate)
	if err != nil {
		return fmt.Errorf("error parsing app registration template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("error executing app registration template: %w", err)
	}

	// Find the position to insert the new code (after urlShortenerController)
	lines := strings.Split(string(content), "\n")
	var newLines []string
	inserted := false

	for _, line := range lines {
		newLines = append(newLines, line)

		// Look for the line with userController initialization
		if strings.Contains(line, "userController := http.NewUserController") && !inserted {
			// Insert the new module registration after this line
			newLines = append(newLines, buf.String())
			inserted = true
		}

		// Also update the RouteConfig struct
		if strings.Contains(line, "UserController *http.UserController") {
			// Add the new controller field to RouteConfig
			routeTmpl, _ := template.New("routeConfigField").Parse(routeConfigFieldTemplate)
			var routeBuf bytes.Buffer
			_ = routeTmpl.Execute(&routeBuf, data)
			newLines = append(newLines, routeBuf.String())
		}

		// Update the RouteConfig initialization
		if strings.Contains(line, "UserController: userController,") {
			routeInitTmpl, _ := template.New("routeConfigInit").Parse(routeConfigInitTemplate)
			var routeInitBuf bytes.Buffer
			_ = routeInitTmpl.Execute(&routeInitBuf, data)
			newLines = append(newLines, routeInitBuf.String())
		}
	}

	// Write back to file
	updatedContent := strings.Join(newLines, "\n")
	// #nosec G306 -- File permissions 0644 are acceptable for source code files
	if err := os.WriteFile(filePath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("error writing updated app.go: %w", err)
	}

	logInfo(fmt.Sprintf("Updated: %s", filePath))
	return nil
}

func updateRouteGo(data TemplateData) error {
	filePath := "internal/delivery/http/route/route.go"

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading route.go: %w", err)
	}

	// Parse the template
	tmpl, err := template.New("routeGroup").Parse(routeGroupTemplate)
	if err != nil {
		return fmt.Errorf("error parsing route group template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("error executing route group template: %w", err)
	}

	// Find the position to insert the new route group (before the closing brace)
	lines := strings.Split(string(content), "\n")
	var newLines []string

	for i, line := range lines {
		newLines = append(newLines, line)

		// Look for the last route group and insert after it
		if strings.Contains(line, "userGroup.Delete(\"/account\", c.UserController.DeleteAccount)") && i < len(lines)-1 {
			// Insert the new route group
			newLines = append(newLines, "")
			newLines = append(newLines, buf.String())
		}
	}

	// Write back to file
	updatedContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(filePath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("error writing updated route.go: %w", err)
	}

	logInfo(fmt.Sprintf("Updated: %s", filePath))
	return nil
}

// ============== MIGRATE RENAME FUNCTIONS ==============

type MigrationFile struct {
	Path     string
	FullName string
	Name     string
	Number   int
}

func runMigrateRename() {
	migrationsDir := "db/migrations"

	files, err := getMigrationFiles(migrationsDir)
	if err != nil {
		logError(fmt.Sprintf("Error reading migration files: %v", err))
		os.Exit(1)
	}

	if len(files) == 0 {
		logWarning("No migration files found!")
		return
	}

	fmt.Println()
	fmt.Printf("%s%s%s\n", ColorBold, ColorCyan, "=========================================")
	fmt.Printf("%s%s%s\n", ColorBold, ColorCyan, "     Migration File Renamer")
	fmt.Printf("%s%s%s\n", ColorBold, ColorCyan, "=========================================")
	fmt.Println()
	fmt.Printf("%sAvailable migration files:%s\n", ColorGreen, ColorReset)
	fmt.Println()

	for i, file := range files {
		fmt.Printf("  %s%3d.%s %s\n", ColorBold, i+1, ColorReset, file.FullName)
	}

	fmt.Println()
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Enter the %sNUMBER%s of file to move: ", ColorBold, ColorReset)
	input, _ := reader.ReadString('\n')
	fileNum, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || fileNum < 1 || fileNum > len(files) {
		logError("Invalid file number!")
		return
	}

	selectedFile := files[fileNum-1]
	fmt.Printf("\n%sSelected:%s %s%s%s\n\n", ColorBold, ColorReset, ColorYellow, selectedFile.FullName, ColorReset)

	fmt.Printf("Enter new %sPOSITION%s (1-%d): ", ColorBold, ColorReset, len(files))
	input, _ = reader.ReadString('\n')
	newPos, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || newPos < 1 || newPos > len(files) {
		logError("Invalid position!")
		return
	}

	if newPos == fileNum {
		logWarning(fmt.Sprintf("File already in position %d", newPos))
		return
	}

	fmt.Printf("\n%sMoving%s '%s%s%s' from position %s%d%s to %s%d%s...\n\n",
		ColorBold, ColorReset, ColorYellow, selectedFile.FullName, ColorReset,
		ColorBold, fileNum, ColorReset, ColorBold, newPos, ColorReset)

	// Calculate what will happen
	if newPos < fileNum {
		fmt.Printf("%sFiles will be shifted RIGHT:%s\n", ColorCyan, ColorReset)
		for i := newPos - 1; i < fileNum-1; i++ {
			fmt.Printf("  %s->%s %s%s%s %s->%s %s%06d%s\n",
				ColorCyan, ColorReset, ColorGreen, files[i].Name, ColorReset,
				ColorCyan, ColorReset, ColorBold, i+2, ColorReset)
		}
		fmt.Printf("  %s->%s %s%06d%s %s->%s %s%06d%s %s(target)%s\n",
			ColorCyan, ColorReset, ColorGreen, selectedFile.Number, ColorReset,
			ColorCyan, ColorReset, ColorBold, newPos, ColorReset, ColorYellow, ColorReset)
	} else {
		fmt.Printf("%sFiles will be shifted LEFT:%s\n", ColorCyan, ColorReset)
		for i := fileNum; i < newPos; i++ {
			fmt.Printf("  %s->%s %s%s%s %s->%s %s%06d%s\n",
				ColorCyan, ColorReset, ColorGreen, files[i].Name, ColorReset,
				ColorCyan, ColorReset, ColorBold, i, ColorReset)
		}
		fmt.Printf("  %s->%s %s%06d%s %s->%s %s%06d%s %s(target)%s\n",
			ColorCyan, ColorReset, ColorGreen, selectedFile.Number, ColorReset,
			ColorCyan, ColorReset, ColorBold, newPos, ColorReset, ColorYellow, ColorReset)
	}

	fmt.Print("\nProceed with rename? [y/N]: ")
	input, _ = reader.ReadString('\n')
	confirm := strings.TrimSpace(strings.ToLower(input))

	if confirm != "y" {
		logWarning("Aborted.")
		return
	}

	logInfo("Executing rename...")
	fmt.Println()

	// PERBAIKAN: Buat copy dari file info sebelum mulai rename
	fileInfos := make([]struct {
		oldNumber int
		name      string
		upPath    string
		downPath  string
	}, len(files))

	for i, f := range files {
		fileInfos[i].oldNumber = f.Number
		fileInfos[i].name = f.Name
		fileInfos[i].upPath = f.Path
		fileInfos[i].downPath = strings.Replace(f.Path, ".up.sql", ".down.sql", 1)
	}

	// Move selected file to temp first
	tempUp := filepath.Join(os.TempDir(), "migrate_temp.up.sql")
	tempDown := filepath.Join(os.TempDir(), "migrate_temp.down.sql")

	selectedUp := fileInfos[fileNum-1].upPath
	selectedDown := fileInfos[fileNum-1].downPath

	if err := os.Rename(selectedUp, tempUp); err != nil {
		logError(fmt.Sprintf("Error moving file to temp: %v", err))
		return
	}
	if err := os.Rename(selectedDown, tempDown); err != nil {
		logError(fmt.Sprintf("Error moving file to temp: %v", err))
		// Restore the up file
		_ = os.Rename(tempUp, selectedUp)
		return
	}

	// Shift files
	if newPos < fileNum {
		// Shift RIGHT: move files from newPos-1 to fileNum-2
		for i := fileNum - 2; i >= newPos-1; i-- {
			oldUp := fileInfos[i].upPath
			oldDown := fileInfos[i].downPath
			newName := fmt.Sprintf("%06d_%s", i+2, fileInfos[i].name)
			newUp := filepath.Join(migrationsDir, newName+".up.sql")
			newDown := filepath.Join(migrationsDir, newName+".down.sql")

			if err := os.Rename(oldUp, newUp); err != nil {
				logError(fmt.Sprintf("Error renaming %s: %v", oldUp, err))
				return
			}
			if err := os.Rename(oldDown, newDown); err != nil {
				logError(fmt.Sprintf("Error renaming %s: %v", oldDown, err))
				return
			}
			logInfo(fmt.Sprintf("Renamed: %06d_%s -> %06d_%s", i+1, fileInfos[i].name, i+2, fileInfos[i].name))
		}
	} else {
		// Shift LEFT: move files from fileNum to newPos-1
		for i := fileNum; i < newPos; i++ {
			oldUp := fileInfos[i].upPath
			oldDown := fileInfos[i].downPath
			newName := fmt.Sprintf("%06d_%s", i, fileInfos[i].name)
			newUp := filepath.Join(migrationsDir, newName+".up.sql")
			newDown := filepath.Join(migrationsDir, newName+".down.sql")

			if err := os.Rename(oldUp, newUp); err != nil {
				logError(fmt.Sprintf("Error renaming %s: %v", oldUp, err))
				return
			}
			if err := os.Rename(oldDown, newDown); err != nil {
				logError(fmt.Sprintf("Error renaming %s: %v", oldDown, err))
				return
			}
			logInfo(fmt.Sprintf("Renamed: %06d_%s -> %06d_%s", i+1, fileInfos[i].name, i, fileInfos[i].name))
		}
	}

	// Move temp file to final position
	finalName := fmt.Sprintf("%06d_%s", newPos, fileInfos[fileNum-1].name)
	finalUp := filepath.Join(migrationsDir, finalName+".up.sql")
	finalDown := filepath.Join(migrationsDir, finalName+".down.sql")

	if err := os.Rename(tempUp, finalUp); err != nil {
		logError(fmt.Sprintf("Error moving temp file to final: %v", err))
		return
	}
	if err := os.Rename(tempDown, finalDown); err != nil {
		logError(fmt.Sprintf("Error moving temp file to final: %v", err))
		return
	}

	logSuccess(fmt.Sprintf("Moved: %s -> %s", fileInfos[fileNum-1].name, finalName))
	logSuccess("Rename complete!")
	fmt.Println()
	fmt.Printf("%sNew order:%s\n", ColorGreen, ColorReset)
	fmt.Println()

	files, _ = getMigrationFiles(migrationsDir)
	for i, file := range files {
		fmt.Printf("  %s%3d.%s %s\n", ColorBold, i+1, ColorReset, file.FullName)
	}
}

func getMigrationFiles(dir string) ([]MigrationFile, error) {
	var files []MigrationFile

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		// Extract number and name from filename
		// Format: 000001_name.up.sql
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}

		num, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(parts[1], ".up.sql")

		files = append(files, MigrationFile{
			Path:     fullPath,
			FullName: entry.Name(),
			Name:     name,
			Number:   num,
		})
	}

	return files, nil
}

// ============== MAIN MENU ==============

func main() {
	fmt.Println()
	fmt.Printf("%s%s%s\n", ColorBold, ColorPurple, "=========================================")
	fmt.Printf("%s%s%s\n", ColorBold, ColorPurple, "     Development Tools")
	fmt.Printf("%s%s%s\n", ColorBold, ColorPurple, "=========================================")
	fmt.Println()
	fmt.Printf("%sSelect an option:%s\n", ColorBold, ColorReset)
	fmt.Println()
	fmt.Printf("  %s1.%s Generate Boilerplate\n", ColorBold, ColorReset)
	fmt.Printf("  %s2.%s Migrate Rename\n", ColorBold, ColorReset)
	fmt.Printf("  %s3.%s Exit\n", ColorBold, ColorReset)
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Choose option (1/2/3): ")
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(input)

	fmt.Println()

	switch choice {
	case "1":
		runGenerate()
	case "2":
		runMigrateRename()
	case "3":
		fmt.Println("Goodbye!")
	default:
		logError("Invalid option")
	}
}
