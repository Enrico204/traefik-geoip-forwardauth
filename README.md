# Traefik GeoIP

This project is a tiny server for [Traefik ForwardAuth](https://doc.traefik.io/traefik/middlewares/http/forwardauth/) middleware implementing a allowlist/blocklist for IPs based on MaxMind data.

Currently, it's able to block IPs based on country of origin.

It has been tested with GeoLite2, however it should work also with GeoIP2 thanks to the library I'm using: https://github.com/oschwald/geoip2-golang

## Usage

```
Usage of traefik-geoip-forwardauth:
  -action string
        Action on countries. If "allow", only those countries are allowed, others are blocked.
        If "block", only countries are blocked, others are allowed. (default "allow")
  -allow-empty-countries
        Whether to allow the request on empty results in country field (default block)
  -countries string
        Comma separated ISO country codes to allow or block (see action flag) (default "IT")
  -db string
        Database path (default "GeoLite2-Country.mmdb")
  -db-refresh-every duration
        After this period of time, the database file is re-read. (default 1h0m0s)
  -debug
        Debug mode (log verbose)
  -web-listen string
        HTTP Listener IP address and port (default ":8080")
  -web-timeout duration
        Timeout when reading/writing HTTP (default 30s)

```

This software does not implement downloading GeoIP2/GeoLite2 databases. There are official tools for this, like the `maxmindinc/geoipupdate` Docker image.

## License

See [LICENSE](LICENSE) file.
