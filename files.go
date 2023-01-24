package pomegranate

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// IngestMigrations reads all the migrations in the given directory and writes
// them to a Go source file in the same directory.  The generateTag argument
// determines whether the new Go file will contain a "//go:generate" comment to
// tag it for automatic regeneration by "go generate".
func IngestMigrations(dir, goFile, packageName string, generateTag bool) error {
	migs, err := ReadMigrationFiles(dir)
	if err != nil {
		return err
	}
	err = writeGoMigrations(dir, goFile, packageName, migs, generateTag)
	if err != nil {
		return err
	}
	fmt.Printf("Migrations written to %s\n", path.Join(dir, goFile))
	return nil
}

// InitMigration creates a new 00001_init migration in the given directory.
// This migration will contain the SQL commands necessary to create the
// migration_state table.
func InitMigration(dir string) error {
	name := makeStubName(1, "init")
	forwardSQL := fmt.Sprintf(initForwardTmpl, name)
	backwardSQL := fmt.Sprintf(initBackwardTmpl, name)
	err := writeStubs(dir, name, forwardSQL, backwardSQL)
	return err
}

// InitMigrationTimestamp creates a new {timestamp}_init migration in the given
// directory. This migration will contain the SQL commands necessary to create
// the `migration_state` table.
func InitMigrationTimestamp(dir string, timestamp time.Time) error {
	intTimestamp, err := strconv.Atoi(timestamp.Format(timestampFormat))
	if err != nil {
		return fmt.Errorf("error creating timestamp on init migration: %v", err)
	}
	name := makeStubName(intTimestamp, "init")
	forwardSQL := fmt.Sprintf(initForwardTmpl, name)
	backwardSQL := fmt.Sprintf(initBackwardTmpl, name)
	err = writeStubs(dir, name, forwardSQL, backwardSQL)
	if err != nil {
		return fmt.Errorf("error making init migration: %v", err)
	}
	return nil
}

// NewMigration creates a new directory containing forward.sql and backward.sql
// stubs.  The directory created will use the name provided to the function,
// prepended by an auto-incrementing zero-padded number.
func NewMigration(dir, name string) error {
	names, err := getMigrationDirectoryNames(OsDir(dir))
	if err != nil {
		return fmt.Errorf("error making new migration: %v", err)
	}
	latestNum, err := getLatestMigrationFileNumber(names)
	if err != nil {
		return fmt.Errorf("error making new migration: %v", err)
	}
	newName := makeStubName(latestNum+1, name)
	forwardSQL := fmt.Sprintf(forwardTmpl, newName)
	backwardSQL := fmt.Sprintf(backwardTmpl, newName)
	err = writeStubs(dir, newName, forwardSQL, backwardSQL)
	if err != nil {
		return fmt.Errorf("error making new migration: %v", err)
	}
	return nil
}

// NewMigrationTimestamp creates a new directory containing forward.sql and
// backward.sql stubs.  The directory created will use the name provided to the
// function, prepended by a timestamp formatted with `YYYYMMDDhhmmss`
// (i.e. `20060102150405`).
func NewMigrationTimestamp(dir, name string, timestamp time.Time) error {
	intTimestamp, err := strconv.Atoi(timestamp.Format(timestampFormat))
	if err != nil {
		return fmt.Errorf("error creating timestamp on new migration: %v", err)
	}
	newName := makeStubName(intTimestamp, name)
	forwardSQL := fmt.Sprintf(forwardTmpl, newName)
	backwardSQL := fmt.Sprintf(backwardTmpl, newName)
	err = writeStubs(dir, newName, forwardSQL, backwardSQL)
	if err != nil {
		return fmt.Errorf("error making new migration: %v", err)
	}
	return nil
}

/*
		ReadMigrationFs allows one to embed an entire migration folder using the [embed] package.

	    go:embed migrations-dir
	    var migrationDir embed.FS
	    ...
	    migrations, err := ReadMigrationFs(migrationDir)

	  It is expected that  migrations is a *directory*, and it will be treated as such"
*/
func ReadMigrationFS(migFolder fs.ReadDirFS) ([]Migration, error) {
	names, err := getMigrationDirectoryNames(migFolder)
	if err != nil {
		return nil, err
	}

	migs := []Migration{}
	for _, name := range names {
		m, err := readMigration(migFolder, name)
		fmt.Println(m, err)
		if err != nil {
			return nil, err
		}
		migs = append(migs, m)
	}
	return migs, nil
}

// ReadMigrationFiles reads all the migration files in the given directory and
// returns an array of Migration objects.
func ReadMigrationFiles(dir string) ([]Migration, error) {
	return ReadMigrationFS(OsDir(dir))
}

// return a list of subdirs that match our pattern
func getMigrationDirectoryNames(dir fs.ReadDirFS) ([]string, error) {
	names := []string{}
	files, err := dir.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("error listing migration files: %w", err)
	}

	for _, file := range files {
		name := file.Name()
		if err != nil {
			return nil, err
		}
		if file.IsDir() && isMigration(name) {
			names = append(names, name)
		}
	}
	return names, nil
}

func getLatestMigrationFileNumber(names []string) (int, error) {
	if len(names) == 0 {
		return 0, nil
	}
	last := names[len(names)-1]
	parts := strings.Split(last, "_")
	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("error getting migration number: %v", err)
	}
	return num, nil
}

func writeStubs(dir, name, forwardSQL, backwardSQL string) error {
	newFolder := path.Join(dir, name)
	err := os.Mkdir(newFolder, 0755)
	if err != nil {
		return fmt.Errorf("error creating migration directory %s: %v", newFolder, err)
	}

	err = ioutil.WriteFile(path.Join(newFolder, "forward.sql"), []byte(forwardSQL), 0644)
	if err != nil {
		return fmt.Errorf("error writing migration file: %v", err)
	}
	err = ioutil.WriteFile(path.Join(newFolder, "backward.sql"), []byte(backwardSQL), 0644)
	if err != nil {
		return fmt.Errorf("error writing migration file: %v", err)
	}
	fmt.Printf("Migration stubs written to %s\n", newFolder)
	return nil
}

func makeStubName(numPart int, namePart string) string {
	return fmt.Sprintf("%s_%s", zeroPad(numPart, leadingDigits), namePart)
}

// little utility to read the contents of a list of file names into
// an array of strings which contains the contents.
func readFileArray(fileNames []string) ([]string, error) {
	files := []string{}

	// sort the input array.  This is so fileName_a, fileName _b are sorted in the correct order
	sort.Strings(fileNames)

	for _, fileName := range fileNames {
		bytes, err := ioutil.ReadFile(fileName)
		if err != nil {
			return files, err
		}
		files = append(files, string(bytes))
	}
	return files, nil
}

// reads the directory containing the folder specified by name.
// reads all the contents of the file into a Migration.
// searches directory for all file names containing either "forward"
// dir is the root directory
// name is the name of the migration folder
func readMigration(dir fs.ReadDirFS, migrationName string) (Migration, error) {
	m := Migration{Name: migrationName, ForwardSQL: []string{}, BackwardSQL: []string{}}

	migrationDirs, err := dir.ReadDir(migrationName)
	if err != nil {
		return m, fmt.Errorf("Unable to list directory: %w", err)
	}

	readEntry := func(sqlFilename string) string {
		// The errors here are so that we give up when we cannot read from a fs.ReadDirFS.
		// these errors are things like `no accces to write` or similar.
		f, err := dir.Open(path.Join(migrationName, sqlFilename))
		panicOnError(err, "Unable to open %q: %w", sqlFilename, err)
		defer f.Close()
		b, err := io.ReadAll(f) // we get an error here if we cannot read bytes
		panicOnError(err, "Unable to read %q: %w", sqlFilename, err)
		return string(b)
	}

	for _, migration := range migrationDirs { // iterate over all the migration folders
		if n := migration.Name(); strings.HasSuffix(n, ".sql") { // looking for "*.sql" files
			if strings.Contains(n, "forward") {
				m.ForwardSQL = append(m.ForwardSQL, readEntry(n))
				continue
			}
			if strings.Contains(n, "backward") {
				m.BackwardSQL = append(m.BackwardSQL, readEntry(n))
				continue
			}
		}
	}
	return m, nil
}

func writeGoMigrations(dir string, goFile string, packageName string, migs []Migration, generateTag bool) error {
	tmpl, err := template.New("migrations").Parse(srcTmpl)
	if err != nil {
		return err
	}

	tmplData := srcContext{
		PackageName: packageName,
		Migrations:  migs,
		GenerateTag: generateTag,
		GoFile:      path.Base(goFile),
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, tmplData)
	if err != nil {
		return err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}

	fname := path.Join(dir, goFile)
	return ioutil.WriteFile(fname, formatted, 0644)
}

func zeroPad(num, digits int) string {
	return fmt.Sprintf("%"+fmt.Sprintf("0%dd", digits), num)
}

var migrationPattern = regexp.MustCompile(fmt.Sprintf(`^[\d]{%d,}_.*$`, leadingDigits))

func isMigration(dir string) bool {
	return migrationPattern.MatchString(dir)
}
