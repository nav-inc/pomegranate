package pomegranate

import (
	"embed"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWriteInitMigration(t *testing.T) {
	dir, _ := ioutil.TempDir(".", "pmgtest")
	defer os.RemoveAll(dir)
	err := InitMigration(dir)
	assert.Nil(t, err)
	f, _ := ioutil.ReadFile(path.Join(dir, "00001_init", "forward.sql"))
	assert.Contains(t,
		string(f),
		"INSERT INTO migration_state(name) VALUES ('00001_init');",
	)
	b, _ := ioutil.ReadFile(path.Join(dir, "00001_init", "backward.sql"))
	assert.Contains(t,
		string(b),
		"Will not roll back 00001_init.",
	)
}

func TestWriteInitMigrationTimestamp(t *testing.T) {
	dir, _ := ioutil.TempDir(".", "pmgtest")
	defer os.RemoveAll(dir)
	err := InitMigrationTimestamp(dir, time.Date(2018, 11, 6, 12, 34, 56, 0, time.UTC))
	assert.Nil(t, err)
	f, _ := ioutil.ReadFile(path.Join(dir, "20181106123456_init", "forward.sql"))
	assert.Contains(t,
		string(f),
		"INSERT INTO migration_state(name) VALUES ('20181106123456_init');",
	)
	b, _ := ioutil.ReadFile(path.Join(dir, "20181106123456_init", "backward.sql"))
	assert.Contains(t,
		string(b),
		"Will not roll back 20181106123456_init.",
	)
}

func TestWriteNewMigration(t *testing.T) {
	dir, _ := ioutil.TempDir(".", "pmgtest")
	defer os.RemoveAll(dir)
	name := "foo"
	err := NewMigration(dir, name)
	assert.Nil(t, err)
	f, _ := ioutil.ReadFile(path.Join(dir, "00001_"+name, "forward.sql"))
	assert.Contains(t,
		string(f),
		fmt.Sprintf("INSERT INTO migration_state(name) VALUES ('00001_%s');",
			name),
	)
	b, _ := ioutil.ReadFile(path.Join(dir, "00001_"+name, "backward.sql"))
	assert.Contains(t,
		string(b),
		fmt.Sprintf("DELETE FROM migration_state WHERE name='00001_%s';", name),
	)
}

func TestAutoNumber(t *testing.T) {
	dir, _ := ioutil.TempDir(".", "pmgtest")
	defer os.RemoveAll(dir)
	NewMigration(dir, "foo") // 00001_foo
	name := "bar"
	err := NewMigration(dir, name)
	assert.Nil(t, err)
	f, _ := ioutil.ReadFile(path.Join(dir, "00002_"+name, "forward.sql"))
	assert.Contains(t,
		string(f),
		fmt.Sprintf("INSERT INTO migration_state(name) VALUES ('00002_%s');",
			name),
	)
	b, _ := ioutil.ReadFile(path.Join(dir, "00002_"+name, "backward.sql"))
	assert.Contains(t,
		string(b),
		fmt.Sprintf("DELETE FROM migration_state WHERE name='00002_%s';", name),
	)
}

func TestNewMigrationTimestamp(t *testing.T) {
	dir, _ := ioutil.TempDir(".", "pmgtest")
	defer os.RemoveAll(dir)
	name := "foo"
	err := NewMigrationTimestamp(dir, name, time.Date(2018, 11, 6, 12, 34, 56, 0, time.UTC))
	assert.Nil(t, err)
	f, _ := ioutil.ReadFile(path.Join(dir, "20181106123456_foo", "forward.sql"))
	assert.Contains(t,
		string(f),
		"INSERT INTO migration_state(name) VALUES ('20181106123456_foo');",
	)
	b, _ := ioutil.ReadFile(path.Join(dir, "20181106123456_foo", "backward.sql"))
	assert.Contains(t,
		string(b),
		"DELETE FROM migration_state WHERE name='20181106123456_foo';",
	)
}

func TestReadMigrations(t *testing.T) {
	dir, _ := ioutil.TempDir(".", "pmgtest")
	defer os.RemoveAll(dir)
	m1 := path.Join(dir, "00001_foo")
	m2 := path.Join(dir, "00002_bar")
	m3 := path.Join(dir, "other_dir") // should be excluded from results
	m4 := path.Join(dir, "20181106123456_baz")
	m5 := path.Join(dir, "00005_sos")
	os.Mkdir(m1, 0755)
	os.Mkdir(m2, 0755)
	os.Mkdir(m3, 0755)
	os.Mkdir(m4, 0755)
	os.Mkdir(m5, 0755)
	ioutil.WriteFile(path.Join(m1, "forward.sql"), []byte("m1 forward"), 0644)
	ioutil.WriteFile(path.Join(m1, "backward.sql"), []byte("m1 backward"), 0644)
	ioutil.WriteFile(path.Join(m2, "forward.sql"), []byte("m2 forward"), 0644)
	ioutil.WriteFile(path.Join(m2, "backward.sql"), []byte("m2 backward"), 0644)
	ioutil.WriteFile(path.Join(m3, "forward.sql"), []byte("m3 forward"), 0644)
	ioutil.WriteFile(path.Join(m3, "backward.sql"), []byte("m3 backward"), 0644)
	ioutil.WriteFile(path.Join(m4, "forward.sql"), []byte("m4 forward"), 0644)
	ioutil.WriteFile(path.Join(m4, "backward.sql"), []byte("m4 backward"), 0644)
	ioutil.WriteFile(path.Join(m5, "forward_1.sql"), []byte("m5 forward"), 0644)
	ioutil.WriteFile(path.Join(m5, "forward_2.sql"), []byte("m5 forward2"), 0644)
	ioutil.WriteFile(path.Join(m5, "backward.sql"), []byte("m5 backward"), 0644)

	expected := []Migration{
		{
			Name:        "00001_foo",
			ForwardSQL:  []string{"m1 forward"},
			BackwardSQL: []string{"m1 backward"},
		},
		{
			Name:        "00002_bar",
			ForwardSQL:  []string{"m2 forward"},
			BackwardSQL: []string{"m2 backward"},
		},
		{
			Name:        "00005_sos",
			ForwardSQL:  []string{"m5 forward", "m5 forward2"},
			BackwardSQL: []string{"m5 backward"},
		},
		{
			Name:        "20181106123456_baz",
			ForwardSQL:  []string{"m4 forward"},
			BackwardSQL: []string{"m4 backward"},
		},
	}
	migs, err := ReadMigrationFiles(dir)
	if err != nil {
		fmt.Println(err)
	}
	assert.Nil(t, err)
	assert.Equal(t, expected, migs)
}

func TestIngestMigrations(t *testing.T) {
	dir, _ := ioutil.TempDir(".", "pmgtest")
	defer os.RemoveAll(dir)
	NewMigration(dir, "foo") // 00001_foo
	NewMigration(dir, "bar") // 00002_bar
	err := IngestMigrations(dir, "testmigrations.go", "somepackage", true)
	if err != nil {
		fmt.Println(err)
	}
	assert.Nil(t, err)
	f, _ := ioutil.ReadFile(path.Join(dir, "testmigrations.go"))
	contents := string(f)
	assert.Contains(t, contents, "package somepackage")
	assert.Contains(
		t,
		contents,
		"//go:generate pmg ingest -package somepackage -gofile testmigrations.go",
	)

	// also check disabling "go generate" tag
	err = IngestMigrations(dir, "testmigrations.go", "somepackage", false)
	assert.Nil(t, err)
	f, _ = ioutil.ReadFile(path.Join(dir, "testmigrations.go"))
	contents = string(f)
	assert.NotContains(
		t,
		contents,
		"//go:generate",
	)
}

//go:embed fixtures/embed
var embedded embed.FS
var testMigrations = FromEmbed(embedded, "fixtures/embed")

func TestReadMigrationFS(t *testing.T) {
	tests := []struct {
		name      string
		migFolder fs.ReadDirFS
		want      []Migration
		wantErr   bool
	}{
		{
			"Using OSPath",
			OsDir("fixtures/embed"),
			[]Migration{
				{
					Name: "00001_init",
					ForwardSQL: []string{
						"BEGIN;\nCREATE TABLE migration_state (\n\tname TEXT NOT NULL,\n\ttime TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,\n\twho TEXT DEFAULT CURRENT_USER NOT NULL,\n\tPRIMARY KEY (name)\n);\n\nCREATE TABLE migration_log (\n  id SERIAL PRIMARY KEY,\n  time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),\n  name TEXT NOT NULL,\n  op TEXT NOT NULL,\n  who TEXT NOT NULL DEFAULT CURRENT_USER\n);\n\nCREATE OR REPLACE FUNCTION record_migration() RETURNS trigger AS $$\nBEGIN\n\tIF TG_OP='DELETE' THEN\n\t\tINSERT INTO migration_log (name, op) VALUES (\n\t\t\tOLD.name,\n\t\t\tTG_OP\n\t\t);\n\t\tRETURN OLD;\n\tELSE\n\t\tINSERT INTO migration_log (name, op) VALUES (\n          NEW.name,\n          TG_OP\n\t\t);\n\t\tRETURN NEW;\n\tEND IF;\nEND;\n$$ language plpgsql;\n\nCREATE TRIGGER record_migration AFTER INSERT OR UPDATE OR DELETE ON migration_state\n  FOR EACH ROW EXECUTE PROCEDURE record_migration();\n\nINSERT INTO migration_state(name) VALUES ('00001_init');\nCOMMIT;\n",
					},
					BackwardSQL: []string{
						"BEGIN;\nCREATE OR REPLACE FUNCTION no_rollback() RETURNS void AS $$\nBEGIN\n  RAISE 'Will not roll back 00001_init.  You must manually drop the migration_state and migration_log tables.';\nEND;\n$$ LANGUAGE plpgsql;\n\nSELECT no_rollback();\nCOMMIT;\n",
					},
				},
				{
					Name: "00002_people",
					ForwardSQL: []string{
						"CREATE TABLE IF NOT EXISTS animal (\n\tid BIGSERIAL PRIMARY KEY,\n\tname TEXT NOT NULL,\n  weight FLOAT NOT NULL\n);\n",
						"CREATE TABLE IF NOT EXISTS pets (\n\tid BIGSERIAL PRIMARY KEY,\n  animal bigint references animal(id),\n\tname TEXT NOT NULL\n);\n",
					},
					BackwardSQL: []string{"DROP TABLE IF EXISTS animal CASCADE;\n", "DROP TABLE IF EXISTS pets;\n"},
				},
			},
			false,
		},
		{
			"Using embed",
			testMigrations,
			[]Migration{
				{
					Name: "00001_init",
					ForwardSQL: []string{
						"BEGIN;\nCREATE TABLE migration_state (\n\tname TEXT NOT NULL,\n\ttime TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,\n\twho TEXT DEFAULT CURRENT_USER NOT NULL,\n\tPRIMARY KEY (name)\n);\n\nCREATE TABLE migration_log (\n  id SERIAL PRIMARY KEY,\n  time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),\n  name TEXT NOT NULL,\n  op TEXT NOT NULL,\n  who TEXT NOT NULL DEFAULT CURRENT_USER\n);\n\nCREATE OR REPLACE FUNCTION record_migration() RETURNS trigger AS $$\nBEGIN\n\tIF TG_OP='DELETE' THEN\n\t\tINSERT INTO migration_log (name, op) VALUES (\n\t\t\tOLD.name,\n\t\t\tTG_OP\n\t\t);\n\t\tRETURN OLD;\n\tELSE\n\t\tINSERT INTO migration_log (name, op) VALUES (\n          NEW.name,\n          TG_OP\n\t\t);\n\t\tRETURN NEW;\n\tEND IF;\nEND;\n$$ language plpgsql;\n\nCREATE TRIGGER record_migration AFTER INSERT OR UPDATE OR DELETE ON migration_state\n  FOR EACH ROW EXECUTE PROCEDURE record_migration();\n\nINSERT INTO migration_state(name) VALUES ('00001_init');\nCOMMIT;\n",
					},
					BackwardSQL: []string{
						"BEGIN;\nCREATE OR REPLACE FUNCTION no_rollback() RETURNS void AS $$\nBEGIN\n  RAISE 'Will not roll back 00001_init.  You must manually drop the migration_state and migration_log tables.';\nEND;\n$$ LANGUAGE plpgsql;\n\nSELECT no_rollback();\nCOMMIT;\n",
					},
				},
				{
					Name: "00002_people",
					ForwardSQL: []string{
						"CREATE TABLE IF NOT EXISTS animal (\n\tid BIGSERIAL PRIMARY KEY,\n\tname TEXT NOT NULL,\n  weight FLOAT NOT NULL\n);\n",
						"CREATE TABLE IF NOT EXISTS pets (\n\tid BIGSERIAL PRIMARY KEY,\n  animal bigint references animal(id),\n\tname TEXT NOT NULL\n);\n",
					},
					BackwardSQL: []string{"DROP TABLE IF EXISTS animal CASCADE;\n", "DROP TABLE IF EXISTS pets;\n"},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadMigrationFS(tt.migFolder)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadMigrationFS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("%#v", &got)
			assert.Equal(t, tt.want, got)
		})
	}
}
