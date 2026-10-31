# P2pool-Scanner-Go

P2pool-Scanner-Go is a simple p2pool scanner written in Go. Unlike many other p2pool scanners, P2pool-Scanner-Go does not need to be run alongside  an existing p2pool node or Core wallet. It is originally developed to be used for the discovery of Vertcoin P2pool nodes, but will also work for other coins such as BTC and LTC with the appropriate  changes made to the config file.

## TODO
[] Add a front facing web interface to display P2pool nodes

## API endpoint
The program also features an API endpoint `/nodes`. The endpoint returns a JSON response with all known public P2pool nodes.
The response features all known information about the node and additionally the geolocation of the node (if setup).

## How to use
To use this scanner you firstly need to [build it](#building). 

Before launching P2pool-Scanner-Go you need to set up a configuration file. See the subsection.

### Config file
Setting up the config file can be as easy as renaming `example_config.json` to `config.json`, and editing the fields as required.

The `BootstrapNodes` and `Port` are the most important parts of this file. These are the first nodes that the scanner will reach and ask for peers. 
It is therefore essential for these nodes to be active. If you don't know about any existing node, you can find them in the P2pool codebase for your project under `p2pool-vtc/p2pool/networks/<coin.py>`.
The port is the default port of the P2pool Node.

`RescanTime` sets the time in minutes before the scanner will rescan over the known nodes. The rescan updates their listed stats, and rescans their peers.

I encourage you to read the following subsections in order to use the p2pool scanner effectively.

#### Forward Domain Lookup
P2pool nodes' peers are always returned as IPv4 addresses. Therefore the option of doing a domain lookup exists. Using domain names looks more professional/cleaner
and if the IP of the node changes for a miner using this scanner, they won't lose the connection on their next startup.

To use this feature, you need to know the domain name of the node.
This information is added to `config.json` under `NodeDomain`, see `example_config.json`.

#### Geolocation
The scanner utilizes a [geolocation service](https://ipstack.com/) to display the geolocation of the public P2pool nodes. 
A free account can be created with (at the time of writing) 100 requests/month. Your access key needs to be added in `config.json` in the `AccessKey` field; `"AcessKey": "?access_key=key"`.

If you don't wish to utilize this service you can leave the fields with the existing text from `example_config.json` or blank. 
This will leave the geolocation fields in the JSON response blank.

Alternatively, another geolocation service can used by modifying the fields inside `GeoLocation` (and possibly util/geo.go) slightly.

#### P2Pool-Go nodes
P2Pool-Go nodes run their own network and talk over libp2p, so they never show up in a legacy node's peer list.
To scan them too, add a `P2PoolGo` block (see `example_config.json`):

- `BootstrapPeers`: libp2p multiaddrs of P2Pool-Go nodes, ending in `/p2p/<peer id>`. The scanner asks each node for its peers over P2Pool-Go's peer exchange (`/p2pool/peers/1.0.0`) and keeps following new peers.
- `WebPort`: the port P2Pool-Go nodes serve `/local_stats` on (9172 by default). Only peers that answer there are listed.

Each node in the output has a `network` field, `legacy` or `p2pool-go`, and a `response_ms` field with how long `/local_stats` took to answer.

### Scanning once
`-once` scans a single time, writes the nodes as JSON and exits, instead of rescanning forever and serving `/nodes`. This suits scheduled jobs such as CI.

```bash
./main -once -config config.json -out nodes.json -timeout 10m
```

- `-config`: config file to read (default `config.json`).
- `-out`: file to write the JSON to, or `-` for stdout (the default). In this mode, logs go to stderr and `debug.log`.
- `-timeout`: stop after this long and write what was found (default `10m`).

## Building
The project utilizes one external dependency [chi](https://github.com/go-chi/chi), to handle HTTP requests.

To build the project from the source, you firstly need to have [Go](https://golang.org/) installed.
Then run the following commands inside the repository clones main folder:

```bash
go mod download
```

```bash
go build main.go
```

This should add `chi` to your GoPATH and produce an executable `main.exe`. Upon launching `main.exe` with a valid [configuration file](#how-to-use), the scanning will begin.

## Donations
If you want to support the development of this program, feel free to donate!

Vertcoin: `VogeggDpe5RL5hafA8prUAMFvzKqb9G2ih`
