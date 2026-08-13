## Perf test

These are k6 performance tests. To build k6 with DNS support, you'll need to build k6 with the DNS plugin.

```
$ xk6 build v0.57.0 --with github.com/grafana/xk6-dns \
```

If you prefer docker install:

```
$ docker run --rm -it -u "$(id -u):$(id -g)" \
   -v "${PWD}:/xk6" grafana/xk6 build v0.57.0 \
  --with github.com/grafana/xk6-dns 

```

Then edit ``tdns.js`` with the server address and test with: ``k6 run tdns.js``

