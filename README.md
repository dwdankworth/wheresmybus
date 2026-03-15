# wheresmybus

A simple CLI tool that tells you how long until your bus arrives, right from the command line.
Uses the [OneBusAway](https://pugetsound.onebusaway.org/) API for King County Metro real-time arrival data.

## Setup

### 1. Get an API key

Sign up at <https://www.pugetsound.onebusaway.org/p/sign-up> to get your OneBusAway API key.

### 2. Find your stop IDs

Search for your home and office bus stops at <https://pugetsound.onebusaway.org/>. The stop ID is shown in the URL (e.g., `1_75403`).

### 3. Configure .env

Copy the example file and fill in your values:

```sh
cp .env.example .env
```

Edit `.env`:

```
OBA_API_KEY=your-api-key-here
HOME_WIFI=MyHomeNetwork
OFFICE_WIFI=MyOfficeNetwork
HOME_STOP_ID=1_75403
OFFICE_STOP_ID=1_12345
```

### 4. Install

```sh
go install github.com/dwdan/wheresmybus@latest
```

Or build locally:

```sh
go build -o wheresmybus .
```

## Usage

```sh
# Auto-detect direction from wifi network
wheresmybus

# Explicitly pick a direction
wheresmybus --direction home
wheresmybus --direction office
```

### How wifi detection works

- Connected to your **home wifi** → shows arrivals at your **office stop** (you're heading to work)
- Connected to your **office wifi** → shows arrivals at your **home stop** (you're heading home)
- Not on either network → use `--direction` flag

### Example output

```
🚌 Arrivals for stop 1_75403:

ROUTE     DESTINATION                     ETA                 STATUS
372E      U-District Station              3 min               📍 1 stops away
67        Northgate Station               5 min               📍 1 stops away
67        Northgate Station               18 min              📍 16 stops away
45        Loyal Heights Greenwood         21 min              📍 15 stops away
372E      U-District Station              22 min              📍 15 stops away
```
