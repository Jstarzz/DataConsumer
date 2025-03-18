# Data Consumer v2.0

[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)](https://github.com/Jstarzz)
[![Go Report Card](https://goreportcard.com/badge/github.com/your-username/your-repo)](https://goreportcard.com/report/github.com/your-username/your-repo)
![GitHub Release](https://github.com/Jstarzz/DataConsumer.git)

A high-performance network data consumer designed for measuring and stress-testing network throughput. This application downloads data concurrently from multiple URLs and provides real-time metrics on data consumption.

## ✨ Features

* **Concurrent Data Downloading:** Utilizes multiple workers to download data from various sources simultaneously, maximizing network bandwidth usage.
* **Interactive Configuration:** Prompts the user for target data rate, verbose logging preference, and the number of workers at startup.
* **Configurable Data Sources:** Uses a configuration file to define the list of URLs to download from.
* **Real-time Metrics:** Displays current data consumption, instantaneous download rate, average rate, peak rate, and elapsed time in the terminal.
* **Target Rate Indication:** Allows specifying a target data consumption rate (for informational purposes).
* **Flexible Duration:** Can run for a specified duration or indefinitely.
* **Verbose Logging:** Provides detailed output for debugging and monitoring.
* **Metrics Persistence:** Saves a summary of the metrics to a JSON file and detailed logs to a CSV file.
* **Graceful Shutdown:** Handles interrupt signals (Ctrl+C) to ensure a clean exit.
* **Command-Line Flags:** Supports command-line flags for additional configuration options.

## 🚀 Getting Started

### 🛠️ Prerequisites

* [Go](https://go.dev/dl/) (version 1.16 or later) installed on your system.

### ⚙️ Installation

1.  Clone the repository:
    ```bash
    git clone <your_repository_url>
    cd dataconsumer
    ```
2.  (Optional) Build the application:
    ```bash
    go build -o dataconsumer
    ```

### 🎮 Usage

1.  Navigate to the project directory in your terminal.
2.  Run the application:
    ```bash
    ./dataconsumer
    ```
    (or `./dataconsumer.exe` on Windows)

    The application will then guide you through the initial setup with interactive prompts:

    ```
    ╔════════════════════════════════════════════╗
    ║                 DATA CONSUMER v2.0                 ║
    ║      High-Performance Network Data Consumer      ║
    ╚════════════════════════════════════════════╝
    Running on linux with 8 CPU cores

    Enter target data consumption rate in MB/min (default: 1024, or press Enter for default):
    Enable verbose logging? (y/N, default: N, or press Enter for default):
    Enter the number of workers to use (default: 8, or press Enter for default):
    Starting data consumption targeting at least 1024 MB/minute
    Data consumption started...
    Press Ctrl+C to stop
    ```

3.  Respond to the prompts to configure the target rate, verbose logging, and the number of workers.

#### Command-Line Flags

You can also use the following command-line flags for additional configuration:

* `-config <path>`: Specifies the path to a JSON configuration file.
* `-duration <minutes>`: Sets the duration to run the consumer in minutes (use `0` for indefinite).
* `-metrics <path>`: Defines the path for saving the metrics summary JSON file (default: `dataconsumer_metrics.json`).
* `-save-interval <seconds>`: Sets the interval in seconds for saving metrics to the JSON file (default: `60`).

**Note:** If a configuration file is provided, the application will load the data sources from it but will still prompt you for the target rate, verbose option, and number of workers interactively.

### ⚙️ Configuration File

The application can load its data sources and default settings from a JSON configuration file. Here's an example `config.json`:

```json
{
  "data_sources": [
    "[https://speed.cloudflare.com/1000mb.bin](https://speed.cloudflare.com/1000mb.bin)",
    "[https://ftp.arnes.si/software/ubuntu-releases/20.04/ubuntu-20.04.3-desktop-amd64.iso](https://ftp.arnes.si/software/ubuntu-releases/20.04/ubuntu-20.04.3-desktop-amd64.iso)"
  ],
  "target_rate": 2048,
  "duration": 60,
  "verbose_logging": false,
  "save_metrics": true,
  "metrics_file": "custom_metrics.json",
  "concurrency_factor": 8,
  "use_randomization": true,
  "request_timeout": 30
}
