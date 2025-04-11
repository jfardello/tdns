## Perf test

This are k6 performace tests in order to build k6 with dns support you'll need to build k6 with th dns plugin.

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