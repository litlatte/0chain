### Serving spec in redoc
-------------------------
   Redoc displays swagger.json just like swagger UI, with some additional styles.
   1. Take a look at the redoc-0chain.html for embedding generated swagger.json
   2. Access it over the node server http://localhost:3001/redoc-0chain.html

#### Serve local in redoc

```shell
    npm i -g redoc-cli
    redoc-cli serve 0chain-docs.yaml
```
see: http://127.0.0.1:8080
