# Pomegranate [![Build Status](https://travis-ci.com/nav-inc/pomegranate.svg?branch=master)](https://travis-ci.com/nav-inc/pomegranate)

Pomegranate is a tool for creating and running schema migrations for the
Postgres database, emphasizing safety and transparency.  All migrations are
run in transactions that will be automatically rolled back if an exception is
raised, ensuring the database is not left in a half-migrated state. Further, the
.sql migration files created by Pomegranate are the exact SQL that will be run
at migration time; you will get the same result whether you call `pmg forward`,
or feed a forward.sql file to `psql`, or attach the forward.sql file to a change
control ticket and have your DBA run it. Postgres's best-in-class support for
[transactional
DDL](https://wiki.postgresql.org/wiki/Transactional_DDL_in_PostgreSQL:_A_Competitive_Analysis)
makes this safety and transparency possible.

Go projects can also use `pmg` to convert their .sql migrations into a `migrations.go` file that
will be compiled into their project and run with the `pomegranate` package.

## Installation

For now, pomegranate has to be built from source:

    $ go install github.com/nav-inc/pomegranate/pmg

## Usage

You can use Pomegranate in two ways:

1. As a standalone command line tool (`pmg`).
2. As a library (`github.com/nav-inc/pomegranate`) in your own Go project.

### Using `pmg`
#### Create initial migration

Use the `pmg init` command to create your first migration, which will be
responsible for creating the `migration_state` table.  It will create the
migration in the current directory, or you can specify a different one with the
`--dir` option.  You can also use the `--ts` option to use timestamps.


    $ pmg init
    Migration stubs written to 00001_init

The `00001_init` directory should now exist, and contain `forward.sql` and
`backward.sql` files.  You don't need to edit these initial migrations.

#### Create more migrations

Migrations containing your own custom changes should be made with the `pmg new`
command:

    $ pmg new add_customers_table
    Migration stubs written to 00002_add_customers_table

Note that the `00002` prefix has been prepended to the name you provided.
Migrations are run in the order they appear in the file system.  The auto
numbering ensures that this ordering is consistent.  You can also use the `--ts`
option to use timestamps.

As with `init`, the `new` command creates `forward.sql` and `backward.sql`
files.  Unlike `init`, these are just stubs.  You will need to edit these files
and add your own commands (e.g. `CREATE TABLE...`).  The stub files try to make
it obvious where your commands should go:

    $ cat 00002_add_customers_table/forward.sql 
    BEGIN;
    -- vvvvvvvv PUT FORWARD MIGRATION CODE BELOW HERE vvvvvvvv

    SELECT 1 / 0; -- delete this line

    -- ^^^^^^^^ PUT FORWARD MIGRATION CODE ABOVE HERE ^^^^^^^^
    INSERT INTO migration_state(name) VALUES ('00002_add_customers_table');
    COMMIT;

The `SELECT 1 / 0;` line in the stub is a safeguard against accidentally running
an empty migration.  You should replace it with your own commands.

Be sure to also add the necessary commands to `backward.sql` to safely roll back
the changes in `forward.sql`, in case you decide they were a bad idea.

For migrations that require statements to be run outside of transactions, the
`forward.sql` and `backward.sql` files can be broken up into `forward_n.sql` and
 `backward_n.sql`, where `n` is a member of the integer sequence starting at 1.
 The migrations will be executed sequentially, starting with `forward_1.sql` or
 `backward_1.sql`. See a multi-file migration example below.

#### Run migrations

Use the `forward` command to run all migrations not yet recorded in the
`migration_state` table.  Pomegranate will connect to the database specified
in the `DATABASE_URL` environment variable, or you can supply a database URL
with the `--dburl` option.

    $ pmg forward
    Connecting to database 'readme' on host ''
    Forward migrations that will be run:
    00001_init
    00002_add_customers_table
    Run these migrations? (y/n) y
    Running 00001_init... Success!
    Running 00002_add_customers_table... Success!
    Done

If you don't want to run all the migrations, you can use the `forwardto` command
instead:

    $ pmg forwardto 00001_init
    Connecting to database 'readme' on host ''
    Forward migrations that will be run:
    00001_init
    Run these migrations? (y/n) y
    Running 00001_init... Success!
    Done

If a migration fails, DON'T PANIC.  Your database should still be in the same
state it was in before that `forward.sql` script was executed. (Unless you put
commands outside the `BEGIN` and `COMMIT` lines.)  Fix the problem in your
script, and run `pmg forward` again.

#### Roll back migrations

Rolling back is done with the `backwardto` command.  This will run the
`backward.sql` file for all migrations that have already been run, up to and
including the one specified in the command.

    $ pmg backwardto 00002_add_customers_table
    Connecting to database 'readme' on host ''
    Backward migrations that will be run:
    00002_add_customers_table
    Run these migrations? (y/n) y
    Running 00002_add_customers_table... Success!
    Done

Unlike going forward, `pmg` does NOT provide a `backward` command that will
migrate all the way back.  You must use `backwardto` and provide an explicit
migration name.

#### View migration state 

The `state` command will show all migrations recorded in the
`migration_state` table:

    $ pmg state 
    Connecting to database 'readme' on host ''
    NAME       | WHEN                                 | WHO
    00001_init | 2018-02-11 20:48:51.827197 -0700 MST | postgres

### Using the pomegranate package in Go

If your project is written in Go, Pomegranate may also be integrated into your
project so that migrations can be included inside your binary program and
executed by it.

The full public interface is documented at
[https://godoc.org/github.com/nav-inc/pomegranate](https://godoc.org/github.com/nav-inc/pomegranate).

#### Using `embed` Package 
The `embed` package was added in `go1.16`, and is a really useful way to embed files inside your go programs.
To use this method, you will need to package your migrations (eg: the folder full of number-prefixed folders)
similar to the following.

```go

//go:embed path/to/migrations
var embedded embed.FS
var migrationFS = pomegranate.FromEmbed(embedded, "path/to/migrations")
...

migrations, err := pomegranate.ReadMigrationFS(migrationFS)

```

This method completely overrides the need for the deprecated method below. 
It will always pick up files and directories that live inside the "path/to/migrations" folder at compile time.

#### Ingest migrations (deprecated)

Use the `pmg ingest` command to turn your .sql migrations into a .go file that
can be compiled into your project.  Run it in the same directory as your
migrations, or use the `--dir` option.

    $ pmg ingest
    Migrations written to migrations.go

By default, migrations are written to `migrations.go` with `package migrations`
at the top.  You can customize the name of the .go file and the package name
inside it with the `--gofile` and `--package` options, respectively.

The file created will have an `All` variable in it containing all your
migrations.

The file will also have a `//go:generate...` tag inside it that will allow to to
re-generate your .go file by running `go generate` in your migrations directory.


#### Run migrations from your code

Use Pomegranate's `MigrateForwardTo` function to run migrations forward.  It
takes four arguments:

- the name that you want to migrate to
- a DB connection
- your ingested migrations
- a boolean flag indicating whether to ask for "y/n" confirmation on the
  command line

~~~
pomegranate.MigrateForwardTo(name, db, migrations.All, true)
~~~

`MigrateBackwardTo` and `GetMigrationState` functions are also available.

#### A complete example

Here's the complete file layout of an extremely simple project that uses Pomegranate:

    $ tree
    .
    ├── migrations
    │   ├── 00001_init
    │   │   ├── backward.sql
    │   │   └── forward.sql
    │   ├── 00002_add_customers_table
    │   │   ├── backward.sql
    │   │   └── forward.sql
    │   └── 00003_add_address_column
    │       ├── backward.sql
    │       └── forward.sql
    └── my_awesome_app.go

In that example, my_awesome_app.go looks like this:

    package main

    import (
      "fmt"
      "os"

      "github.com/nav-inc/my_awesome_app/migrations"
      "github.com/nav-inc/pomegranate"
    )

    func main() {
      db, err := pomegranate.Connect(
        "postgres://postgres@/awesome_app?sslmode=disable")
      if err != nil {
        fmt.Println(err)
        os.Exit(1)
      }
      // passing an empty string as name will run to the latest migration
      pomegranate.MigrateForwardTo("", db, migrations.All, true)
    }

Given the above, you can build my_awesome_app like so:

    $ cd migrations
    $ pmg ingest
    Migrations written to migrations.go
    $ cd ..
    $ go build

And run it like so:

    $ psql -c "CREATE DATABASE awesome_app"
    CREATE DATABASE
    $ ./my_awesome_app 
    Connecting to database 'awesome_app' on host ''
    Forward migrations that will be run:
    00001_init
    00002_add_customers_table
    00003_add_address_column
    Run these migrations? (y/n) y
    Running 00001_init... Success!
    Running 00002_add_customers_table... Success!
    Running 00003_add_address_column... Success!

#### A multi-file migration example

Here's an example of a project that uses Pomegranate and needs to utilize a multi-file migration:

    $ tree
    .
    ├── migrations
    │   ├── 00001_init
    │   │   ├── backward.sql
    │   │   └── forward.sql
    │   ├── 00002_add_customers_table
    │   │   ├── backward.sql
    │   │   └── forward.sql
    │   └── 00003_index_customers_table
    │       ├── backward_1.sql
    |       ├── backward_2.sql
    |       ├── forward_1.sql
    │       └── forward_2.sql
    └── my_awesome_app.go

Let's take a look at `forward_1.sql` and `forward_2.sql`

`forward_1.sql`:

    CREATE INDEX CONCURRENTLY
        customer_index
    ON
        customers (id);

`forward_2.sql`:

    INSERT INTO
        migration_state(name)
    VALUES
        ('00003_index_customers_table');

The reason we need to separate our forward migrations into
two files is because we want to utilize the `CONCURRENTLY`
parameter when creating our index. This parameter will allow
our PostgreSQL to create the index without acquiring a lock
that will prevent our database from performing. This is
especially important in production environments with very
large tables. The downside of the `CONCURRENTLY` parameter
is that it must be run outside of a SQL transaction. We can
do that by isolating the command in `forward_1.sql`. Of
course, we want to keep track of our migration state, so
we put that insert statement into `forward_2.sql`.

Aside from the change addressed above, the rest of the
my_awesome_app.go will be the same:

    package main

    import (
      "fmt"
      "os"

      "github.com/nav-inc/my_awesome_app/migrations"
      "github.com/nav-inc/pomegranate"
    )

    func main() {
      db, err := pomegranate.Connect(
        "postgres://postgres@/awesome_app?sslmode=disable")
      if err != nil {
        fmt.Println(err)
        os.Exit(1)
      }
      // passing an empty string as name will run to the latest migration
      pomegranate.MigrateForwardTo("", db, migrations.All, true)
    }

Given the above, you can build my_awesome_app like so:

    $ cd migrations
    $ pmg ingest
    Migrations written to migrations.go
    $ cd ..
    $ go build

And run it like so:

    $ psql -c "CREATE DATABASE awesome_app"
    CREATE DATABASE
    $ ./my_awesome_app 
    Connecting to database 'awesome_app' on host ''
    Forward migrations that will be run:
    00001_init
    00002_add_customers_table
    00003_index_customers_table
    Run these migrations? (y/n) y
    Running 00001_init... Success!
    Running 00002_add_customers_table... Success!
    Running 00003_index_customers_table... Success!
