# Database Restore and Modification Tool (go-mysql)

This is a Go program for restoring and modifying a MySQL database from a set of SQL files. It provides a convenient way to automate the process of database restoration and post-restore modifications using Docker and SQL scripts.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [License](#license)

## Prerequisites

Before using this tool, make sure you have the following prerequisites in place:

- Docker installed and running on your system.
- A MySQL server running inside a Docker container.
- Access to the SQL files you want to restore and modify.

## Installation

1. Clone this repository to your local machine:

   ```bash
   git clone https://github.com/your-username/your-repo-name.git
   cd your-repo-name
2. You can now use the main binary to restore and modify databases.

## Usage

To use the program, follow these steps:

1.  Make sure you have a MySQL database server running inside a Docker container.

2.  Modify the DB_NAME_MYSQL and DB_PASS_MYSQL constants in the main function of the main.go file. Replace "your_database_name" and "your_database_password" with your actual MySQL database name and password.

3. Run the program:

    ```bash

    ./main <restore-path>

Replace <restore-path> with the path to the directory containing the SQL files you want to restore.

The program will connect to the MySQL database, restore the specified SQL files, and perform post-restore modifications based on the contents of the modify directory.

## Configuration

You can modify the behavior of the program by adjusting the values in the main.go file:

- The number of concurrent workers (maxConcurrent) for database restoration.
- The sorting criteria for SQL files in the modify directory.
- The paths to Docker and the MySQL Docker container, if different from the default values.

Customize these settings to suit your specific requirements.