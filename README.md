🚀 Deploying a Golang REST API on Kubernetes with Helm, Prometheus, and Grafana

# Project & Goals
This project will build a simple quote searcher API using OpenAI in Golang.

## Goals
1. Learn how to set up a golang development environment with a dev vector databse.
   1. Gain familiartiy with `sqlc` for go code generation so we dont have to use or roll a ORM
   2. Set up Air for hot reloading
   3. Set up and use Templ for server side rendered pages
2. Learn about the finer points of vector database data modeling
   1. Trade-off of having a separate table for vectors vs a single table
   2. Vector index setup (HNSW vs IVFFLAT) and what each one means
3. Learn how to use OpenAI to create and store embeddings
4. Learn how to write SQL queries using vectors
5. Learn how to integrate Prometheus to gather endpoint usage and latency metrics
6. Learn how to onboard the app to Kubernetes using Docker Desktop (because production k8s clusters are expensive and I am cheap)
   1. We'll use helm to manage creating the k8s project and manage values
   2. We'll use Age & SOPS to create and manage secrets from the .env file
   3. We'll use a nginx ingress controller, horiz. pod autoscaler, and some services and deployments.
      1. using ingress controller instead of api gateway because we're not covering gRPC (ingress controllers only handle L7 (http/s) traffic, whereas something like kong is an API gateway designed to handle both L7 and L4 traffic (RPC, TCP and UDP))
7. Learn how to add a Grafana dashboard to view metrics

# Local Dev Setup
Starting with [Go Blueprint](https://github.com/Melkeydev/go-blueprint)
`go-blueprint create --name quoteGPT --framework standard-library --driver postgres --advanced false`

Add a Dockerfile. We'll use a multi stage build for faster subsequent builds. 

We're using [Templ](https://templ.guide/) to make some basic web pages to hit our server. To make this work with hot reloading, we'll use [Air](https://github.com/air-verse/air) so we don't have to keep re-compiling and re-building our templates. Make sure you add Templ and Air to your dockerfile or install them with `go get` so they'll be installed as part of the build process in Docker.

Modify the `air.toml` file to run `templ generate` as part of the relaod process so changes to `.templ` files can be re-loaded as you save

```toml
# .air.toml
[build]
cmd = "templ generate && make build"`                # Need to build templates on reload also
exclude_regex = ["_test.go", "_templ.go"]            # Don't want to build when generated _templ.go files are changed
include_ext = ["go", "tpl", "tmpl", "templ", "html"] # Want to build when .templ edits are made
```

Update docker-compose to run the app and database locally. Using docker-compose for local development is generally my go to since its easy to set up and a bunch of networking context isn't required to troubleshoot any issues with the service.

We're going to use `akane/pgvector` as our postgres image. Andrew Kane has done a lot of great work with vector databases in Postgres and this image is basically the postgres image with the `pgvector` extension ready to go. We'll also pull our environment variables from a .env file so everything is managed in one place for local dev. We'll get into secrets later.

Speaking of databases, `go-blueprint` tries to set up some database initialization code. We won't be using that since we'll be using [sqlc](https://sqlc.dev) to generate our database code. Delete the health endpoint in `server/server.go` `server/routes.go` and the `database/database.go` file entirely.

Next we'll use a tool called `sqlc` that translates raw sql queries into go code. This will help us write sql statements that can be executed with the `pgx` package without needing an ORM. We'll just use it to make the db package by hand by installing sqlc with your favorite package manager (I used `brew install sqlc`) and running `sqlc generate -f ./database/sqlc.yaml` Refer to the `sqlc.yaml` for the `out` to see that the generated go files land in the `/database` directory.

Now we need to initialize the connection with `pgx` and pass that to our generated database code so it can use the connection to call the database. We'll just do this in `server/server.go`. 


We're almost ready to test the local development environment. Add this line to your `docker-compose` file. This maps the `schema.sql` file in our `database` directory to the `docker-entrypoint-initdb.d` folder. This folder contains all the SQL scripts that get run at the database initialization. It will only run during the _first_ time the database is brought up, therefore changing this file and expecting those updates to run after bringing up the container for the first time will result in nothing. That means, any table updates or hand-rolled sql will have to be copied over into the container via a database client like [pgAdmin](https://www.pgadmin.org/) or `psql`.

⚠️ If you don't care about persisting data all that much, you can delete the docker volume for the database and re-initialize with new SQL code ⚠️

`- ./database/schema.sql:/docker-entrypoint-initdb.d/schema.sql`

# Local Dev Checks
Run `docker-compose up` and ensure you see the database get initialized and ready to accept connections and that the api is running. 

Call `/hello-world` and make sure you get the expected response `curl http://localhost:8080/hello-world`

Change the returned message in the `hello-world` handler to something different and save the file. You should see a rebuild and when you hit the `/hello-world` endpoint you should see the new message without needing to bounce the container or rebuild.

Exec into the postgres container `exec -it postgres psql -h localhost -U postgres -d postgres` and ensure the table you defined in your `database/schema.sql` is there. You can do that by simply running a `SELECT * FROM <table>;` (dont forget the `;`) and making sure an empty table is there.

🎉 Congratulations, you're ready to write business logic and endpoints and such with a mad decent developer environment.

# Get some dummy data
For this demo I thought it would be fun to make a searchable quote database with OpenAI. That means we need something to search so we'll just get that from [DummyJson's quotes endpoint](https://dummyjson.com/docs/quotes). We'll go and pull in a couple hundred quotes by paging their API and writing them to our database with the vector extension.

Quotes are good for this because they're generally interesting and aren't long enough to require chunking for the embedding process which means fewer tables. There's a couple options we have at this juncture:

* A) Tack a vectors column onto the quotes table
  * Single table with atomicity
  * no joins 
  * if we want to index on the vector column there's overhead on writes to this table
* B) Track vectors on a separate table
  * Now we have to track 2 tables, so there's potential for drift/corruption if theres a logic error
  * Can independently optimize vector query performance and indices
  * Need a join

Since our project is pretty small and significantly read heavy, we can pay the price of indexing on a single table where our users are mostly searching/reading (indexes are computed at write/delete) so we'll just go ahead and use option A.

To get the data, I left a seed function that will pull in the dummy json and run it through OpenAI's embedding model. Uncomment the call out if you need to set up for the first time otherwise youre going to find you have a ton of duplicate data (its not very sophisticated and I just needed something to yank down a bunch of dummy data so don't judge me). It also doesn't discriminate based on existence so running it once, stopping it, and re-starting will result in duplicates.

Pulling about 1100 quotes took around 20k tokens and costs less than a cent to compute all the embeddings. Not bad.

Exec into the postgres container `exec -it postgres psql -h localhost -U postgres -d postgres` and run a `SELECT embedding FROM quotes LIMIT 1;` (dont forget the `;`) and make sure you see an embedding vector (you might have to scroll down a bit);

We now have data to play around with so lets implement the search and find endpoints.

# Basic Fetcher Endpoints
Lets start with a simple list all endpoint to make sure everything is wired up correctly. We're only dealing with about 1000 or so entries so I won't spend a bunch of time discussing pagination and limits and all that. 1000 entries will come out just fine for this exercise.

We can add a handler to the server for `get /quotes` and use the db instance on the server (which is really like a repository) to pull all the quotes out. We'll also add a query param handler so users can search for quotes by a particular author like so `curl localhost:8080/quotes\?author\=Warren%20Buffett` 

⚠️ I don't create embeddings for authors and that makes this search kinda suck. e.g. Warren Buffet returns nothing, because its actually Warren Buffett that you want ⚠️

While we're messing with routes, lets go ahead an add some middleware so we can enable CORS for the frontend. We'll create a middleware file and add a method that takes a `http.HandlerFunc` and returns a `http.HandlerFunc` which is really just the passed in handler func but we get to add some stuff in the middle (hence the name).

```go
func enableCors(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch origin := r.Header.Get("Origin"); origin {
		case "http://localhost":
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "PUT, DELETE, POST, GET, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

```

We can simply wrap our handler with the middleware like so `mux.HandleFunc("/quotes", enableCors(s.ListQuotesHandler))`

# Lets get our magic vector query up
The basic stuff in the last section should be unsurprising. Next we'll enable users to ask questions like "give me some quotes about money management" and the user should see some stuff from Warren Buffett and Bill Gates etc.

This query is going to hit OpenAI (or LLM of choice) to vectorize the query. That embedding can be compared to all the quote embeddings thanks to pgvector. We're going to use cosine similarity.

```sql
-- name: SearchQuotes :many
SELECT (id, author, content) FROM quotes
ORDER BY embedding <=> $1 -- the <=> spaceship operator means use cosine similarity for the comparison. You can also use <-> for L2 norm (pythagorean) or <#> for inner product
LIMIT 5;

CREATE INDEX idx_embedding 
ON quotes 
USING ivfflat (embedding vector_cosine_ops) 
WITH (lists = 100);
```

While we're here lets talk about the index. We're using something called inverted file with flat compression. To understand that, you need to know that when we say "vectors" we're actually talking about coordinates of a word in a universe where instead of 3 dimensions like we have, there are 1536 dimensions. Similar words like King and Man or King and Queen are clustered together in regions. The IVFFlat index works by calculating the centroids of these regions and when the actual search happens, we can see which centroid the query vector most closely aligns with and narrow our search to just that region. 

For example, we can say we want to find a person in a map instead of in a phone book (which is the standard kind of index we're probably used to seeing). If we know the person is American and has a phone number starting with 212 we can start with Manhattan instead of searching every city on the globe.

Hierarchical Navigable Small Worlds (HNSW) is the alternative to IVFFlat and works by constructing a tree that resembles a skip list. That lets us eliminate possibilities really quickly based on the incoming query vector. HNSW indices tend to take up more space and compute power to build, however they are significantly faster than IVFFlat indices and don't suffer from drift like IVFFlat indices do.

⚠️ When updates are made in postgres, IVFFlat indices centroids are not re-computed (the index is not automatically rebuilt) so recall suffers over time with many updates. Since we're not updating frequently this isn't an issue, but it is something to keep in mind for larger projects. ⚠️

This [article](https://tembo.io/blog/vector-indexes-in-pgvector) sums up the guidelines for choosing a indexing algorithm.

* If you care more about index size, then choose IVFFlat.
* If you care more about index build time, then select IVFFlat.
* If you care more about speed, then choose HNSW.
* If you expect vectors to be added or modified, then select HNSW.

Back to the router - we're going to call out to our ready made SQL statement assembler but first we need to make an embedding out of our query. This is how we do that using some code from `embedding/openai.go` 

```go
func (s *Server) searchQuotes(ctx context.Context, query string) ([]interface{}, error) {
	queryEmbed, err := embedding.OpenAI(ctx, query)
	if err != nil {
		return []interface{}{}, err
	}
	return s.DB.SearchQuotes(ctx, pgvector.NewVector(queryEmbed))
}
```

All `embedding.OpenAI` does is call out to the OpenAI API with the text on the query parameter to create another 1536 dimensional embedding vector that we can convert to a pgvector vector for comparison. Postgres will execute a cosine similartiy on each vector and return a list of the nearest neighbors.

Cosine similarity works by drawing lines from an origin point to each embedding location. Since we indexed using IVFFlat, Postgres knows which cluster to point at and which clusters to ignore. From there, we can put a point for our query in the cluster and draw a line to it. Then we can draw lines to every other point in the cluster. The closer embeddings will have small angles between them and therefore are more similar.

# Templates and Client Side Stuff
For this we'll just create a quick input box that forms up the query we just implemented in the last section and returns similar quotes. We could get a list of the authors and let users filter quotes from a list of authors or something like that but in the interest of time I'll leave that as a feature to implement for the reader 😉

I have no strong opinions on where frontend pages should live since I do most of my work on pure frontends anyway so I'll just say create a folder called `/pages` and we'll put all the templ stuff there. We'll just add a page called `search.templ`. If you remember way back, we set up our Air config to look for changes in `.templ` files. That way we can make tweaks and have the frontend build after every change. 

Theres also supposedly a way to set up React with Templ but that is way beyond the scope of this already really long intro so we'll just stick with a basic ass HTML/CSS/Javascript page that gets a query and prints quotes to the screen. Use the included `search.templ` page as an example. Its pretty straightforward. There's a go function that returns some client side html. You add css and javascript in the respective script tags and use document selectors to get the query params, call the API, render out the data, and insert elements.

I didn't format the responses from the database so they come back as arrays of arrays e.g. `[[id, author, quote], [id, author, quote]]` you can format the responses by creating a response struct and marshalling that into json before returning it but for this example we'll skip that step.

# Lets deploy to local kubernetes
Use helm to create the boilerplate kubernetes files `helm create quotegpt`. Then delete everything in `/templates/deployment.yaml` `/templates/service.yaml` `/templates/ingress.yaml` and `/templates/hpa.yaml` because we're using a pretty simplistic setup here. You can also drop `/templates/tests` since we're not doing any testing here.

⚠️ You might also want to add the folder generated by `helm create` to the `exclude_dir` list in `air.toml`

## Secrets
Before we begin we'll generate a kubernetes secret that our deployments can consume. This will just take the `.env` file and create a kubernetes secret yaml from it and add it to the cluster. `kubectl create secret generic postgres-env --from-env-file=.env`

⚠️ This is not a production safe way to do things. You typically want to use something like a cloud secret manager or asymetric encryption that your CI can decrypt to add new secrets to the cluster when secrets change. We're doing the shortcut way for demonstration purposes only.

## Deployments
Deployments tell kubernetes which apps to deploy and a bit about how many replicas we need running in the cluster. Start with this `/templates/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
        - name: api
          imagePullPolicy: IfNotPresent # Lets us use images we have locally
          image: {{ .Values.api.image }}
          ports:
            - containerPort: {{ .Values.api.port }}
          resources:
            requests:
              cpu: "200m"
              memory: "256Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
          envFrom:
            - secretRef:
                name: postgres-env
---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: {{ .Values.postgres.image }}
          ports:
            - containerPort: {{ .Values.postgres.port }}
          volumeMounts:
            - mountPath: /var/lib/postgresql/data
              name: pgdata
            - name: init-sql
              mountPath: /docker-entrypoint-initdb.d/schema.sql
              subPath: schema.sql 
          envFrom:
            - secretRef:
                name: postgres-env
      volumes:
        - name: pgdata
          persistentVolumeClaim:
            claimName: postgres-pvc
        - name: init-sql
          configMap:
            name: postgres-init-sql
```

Notice we have 2 deployments in here `api` and `postgres` separated by a `---` which is yaml for "consider this multiple yaml files in one" (you could use managed postgres if you want - just take the `postgres` deployment out and set up the environment or secrets to point to the managed postgres data such as database, user, password, and so on)

Wherever you see `{{}}` that is telling _helm_ to pull this info from somewhere else. In our case `.Values`. This makes it so when we play around with configurations for HPA or to swap images, we just need to change the `values.yaml` file. 

We specify `resources` for our API because we're going to use the horizontal pod autoscaler. Setting resources like this tell kubernetes we want at least 200m of CPU and 256MB of memory available on a node in order to start the pod, but no pod should ever go beyond 500m CPU and 512MB memory and if we hit that limit we should be bringing up a pod to handle the excess load.

`envFrom` tells kubernetes we want to bring in environment variables from secrets that we'll establish by hand (because this is local) later. If you want to create a production version of this you'd need to configure secrets in the cluster as part of CI where that data is either encrypted here with `age` and decrypted with a private key in CI or CI would need permission to read from a key store like AWS Secrets Manager to create kubernetes secrets that the pods can consume later in the cluster.

In the `postgres` deployment we're specifying a `persistentVolumeClaim` so postgres data can be persisted between restarts if necessary. Again, this is only necessary if you intend to run a pg instance in your cluster. For production apps you really want to use managed databases.

## Jobs
We'll need a one-off job to seed the database. There is a separate "app" in `/cmd/mx/seed.go` (mx is short for maintenance and can hold other one-off scripts). To create a job 

* we'll add a script to `/cmd/mx` (this step is already done)
* add a dockerfile.<name of job> in this case `Dockerfile.seed` to build an image that just has the one-off maintenance job
* build it `docker build -t <name of job>:latest -f Dockerfile.seed .`
* add a `templates/job.yaml` with the following

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Release.Name }}-db-seed-job
  annotations:
    "helm.sh/hook": post-install
    "helm.sh/hook-delete-policy": before-hook-creation
  labels:
    app: {{ .Release.Name }}-db-seed
spec:
  template:
    spec:
      containers:
        - name: db-seeder
          image: {{ .Values.seed.image }}
          envFrom:
            - secretRef:
                name: postgres-env
      restartPolicy: OnFailure
  backoffLimit: {{ .Values.seed.backoffLimit }}
```

and the values like so

```yaml
seed:
  image: quotegpt-seeder:latest
  backoffLimit: 3
```

the annotations in the job tell helm to only run this after the initial installation of the app in the cluster. We could also have `post-upgrade` either by itself or comma separated but we don't want to re-seed every time we update the cluster.

Notice the `envFrom` node. We're using our secret from earlier to get database connection info to call up the database.

Finally to finish setting up the database, we'll need to load all the table creation and extension calls from `/database/schema.sql`. To do so we'll create a config map from the `schema.sql` via `kubectl create configmap postgres-init-sql --from-file=./database/schema.sql`

## Config Maps
A config map is a way to communicate _non sensitive_ config parameters such as log-level or external urls for things like chatgpt. Functionally it works the same as a secret but the information in a config map is transparent so you should not store secrets and keys in here.

Config maps look like so. In this example we're storing a bunch of sql statements into a `schema.sql` file (adding an extension to the key effectively makes it a file).

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-init-sql
data:
  other-data: "some config variables"
  schema.sql: |
    -- Example schema
    CREATE TABLE users (
      id SERIAL PRIMARY KEY,
      name VARCHAR(100),
      email VARCHAR(100)
    );
```

Since we're writing a file, we need to mount it with a volume mount

```yaml
- name: init-sql
  configMap:
    name: postgres-init-sql
```

Reference that volume mount in the `deployment` to specify which directory to load the file into

```yaml
- name: init-sql
  subPath: schema.sql # load this file
  mountPath: /docker-entrypoint-initdb.d/schema.sql # to this location
```

Recall from the docker-compose section that mounting a file into the `docker-entrypoint-initdb.d` folder in the postgres image will cause it to run on database setup. To re-run this we'll need to blow away the whole volume and start over.

## Services
Services define how pods can be reached on the network. We'll define one for the golang api and one for postgres.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api
spec:
  selector:
    app: api
  ports:
    - port: 80
      targetPort: 8080
  type: ClusterIP

---

apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
    - port: 5432
      targetPort: 5432
```

## Persistent Volume Claime
Next we'll define a persistent volume claim. We need this to tell kubernetes we want to persist some data in the cluster and how much memory to reserve for that data. Create a file called `templates/pvc.yaml`. 

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
  annotations:
    "helm.sh/resource-policy": keep  # Prevent Helm from deleting the PVC
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .Values.postgres.storageSize }}
```

## Ingress for the API
Next we'll need an ingress based on nginx to control routing for the API. We could also use an API gateway if we were doing anything with UDP or gRPC but for the purposes of this exercise, an ingress will work fine.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
    - host: {{ .Values.ingress.host }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: api
                port:
                  number: 80
```

What we're basically saying with this is, route anything coming in on port 80 to the `backend` `service` called `api` defined above (i.e. if we hit `localhost/quotes` or `http://localhost/ports`). The `service` will route anything coming to it on port 80 (forwarded from the ingress) to port 8080 in the pod as we set in the `service.spec.ports.targetPort: 8080`

You also need to install an ingress controller into your cluster. Do that with 

```sh
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx -n ingeress-nginx --create-namespace
```

## Horizontal Pod Autoscaler
The horizontal pod autoscaler describes rules for when kubernetes should add more pods to take up more incoming traffic. First we'll need to make sure we're able to collect metrics. Install a metrics server to the kubernetes cluster with `kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml` 

Next we'll add a hpa.yaml file to describe the autoscaling behavior. What this says is create a HPA called `api-hpa` and look to the `deployment` called `api` and autoscale that between the min and max replicas.



```yaml
{{- if .Values.hpa.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api
  minReplicas: {{ .Values.hpa.minReplicas }}
  maxReplicas: {{ .Values.hpa.maxReplicas }}
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ .Values.hpa.targetCPUUtilizationPercentage }}
    {{- if .Values.hpa.targetMemoryUtilizationPercentage }}
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: {{ .Values.hpa.targetMemoryUtilizationPercentage }}
    {{- end }}
{{- end }}
```

Scaling is based on CPU and memory usages according to the following formulae

#### CPU Scaling
`Desired Replicas = Current Replicas × (Current CPU Utilization / Target CPU Utilization)`

#### Memory Scaling
`Desired Replicas = Current Replicas × (Current Memory Utilization / Target Memory Utilization)`

To define what these resource requirements are, add a `resources` node to the api's `deployment`

```yaml
resources:
  requests:
    cpu: "200m"
    memory: "256Mi"
  limits:
    cpu: "500m"
    memory: "512Mi"
```

What this says is the api `requests` 200m CPU and 256Mb of ram. Kubernetes will find a node that has at least that much resources and schedule a new pod there once the existing pods have hit their limits.

`limits` is like a safeguard for the node in the event one of the apps has a meomory leak and begins bogarting resources on the node. Rather than dropping the whole node, kubernetes will throttle or kill a pod for going over its resource limits and try to bring up a new pod in its place in accordance with the min and max replicas described on the HPA. 

#### Resources we Defined

* Values - where we'll control configuration parameters in one place
* Deployments - describes how an app should be deployed (e.g. number of replicas and resource requests and limits) and adds metadata handles so we can reference the pods in other kubernetes concepts like HPAs and Services
  * Deployments will create pods via replica sets (there's no need to make these items individually)
  * Generally speaking apps have a 1:1 pod:container relationship but it's possible to have 1:n pod:container configurations
* Services - describe how pods can be reached inside the cluster via http
* Jobs - one-off data migration or other scripts that need to get called to put the cluster in a usable state
* Ingress - describes how to route traffic from outside the cluster to pods inside the cluster as described in the service
* HPA - describes how/when to add/remove pods based on resource usage and availability
* PVC - describes where to park data that should be kept between cluster or pod restarts. Deployment will point to a PVC for the pods to read/write. 
* Secrets - we created this with a `kubectl` command from `.env` not a `.yaml` file so you can see it with `kubectl describe secret postgres-env`
* Configmap - we created this also with `kubectl` from the `.sql` file and you can confirm its existence with `kubectl describe configmap postgres-init-sql`

Lets build and deploy to our local cluster on docker desktop. 

# Deployment to Docker Desktop
First make sure you have kubernetes enabled in docker desktop. Go to settings (the gear at the top right of the dashboard) and select kubernetes in the left rail and make sure enable kubernetes is ticked. Give it a few minutes to start up if you needed to turn it on.

Next we'll build the image for the app. In case you haven't noticed, Im calling the app `quotegpt` so I'll just run `docker build -t quotegpt:latest .`. Since we're not hosting the image anywhere we wont need to fuss with pushing it up and assigning permissions to pull. Make sure in `Values.yaml` you have under the `api` node `image: quotegpt:latest`

We're able to pull the image from our local host because we set `imagePullPolicy: IfNotPresent` which will see the image present and not attempt to pull from dockerhub or elsewhere which is default behavior.

Next run `helm install quotegpt ./quotegpt` to have helm apply all our templates. Helm will use values and references as described between the `{{ }}` to build kubernetes templates under the hood and use `kubectl apply -f <file>.yaml` for us so we don't have to type a bunch of kubectl commands.

I use [k9s](https://k9scli.io/) to check on my cluster but you can also do `kubectl get <pods|services|deployments etc...>` to see how things are doing.

🤔 I was playing with nginx ingresses prior to making this so my ingress didnt start and I kept getting a bunch of weird "failed to call webhook" errors for nginx. I got around this by running `kubectl rollout restart deployment ingress-nginx-controller -n ingress-nginx`

Once everything runs smoothly you should see all the [resources we created](#### Resources we Defined)

You can now visit `localhost/page` (without the port because nginx is listening to :80) and see our beautiful templ page and start searching quotes.

# LMAO (Logging Monitoring Alerting and Observability)
We're not quite done yet, as its now time to add instrumentation so we can see how well our app is doing. For this we'll need to install prometheus and grafana into the cluster and add some instrumentation middleware to our go app.

## Go App
```sh
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

Add the following to `server/middlware.go`

```go
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Define Prometheus metrics
var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Number of HTTP requests processed",
		},
		[]string{"method", "endpoint"},
	)

	requestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Histogram of latencies for HTTP requests",
			Buckets: prometheus.DefBuckets, // Default buckets: 0.005s, 0.01s, 0.025s, etc.
		},
		[]string{"method", "endpoint"},
	)
)

func init() {
	// Register the metrics with Prometheus
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(requestLatency)
}

func hitCounterMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		requestCount.WithLabelValues(r.Method, r.URL.Path, http.StatusText(recorder.statusCode)).Inc()
	})
}

func latencyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		duration := time.Since(start).Seconds()
		requestLatency.WithLabelValues(r.Method, r.URL.Path, http.StatusText(recorder.statusCode)).Observe(duration)
	})
}
```

Wrap the endpoints of interest with our fancy new middlware

```go
mux.HandleFunc("/quotes", hitCounterMiddleware(latencyMiddleware(enableCors(s.ListQuotesHandler))))
mux.HandleFunc("/quote/{id}", hitCounterMiddleware(latencyMiddleware(enableCors(s.GetQuoteHandler))))
mux.HandleFunc("/page/", hitCounterMiddleware(latencyMiddleware(enableCors(s.PageHandler))))
  ```

expose a `/metrics` endpoint so prometheus has something to hit to get all these metrics.

```go
http.Handle("/metrics", promhttp.Handler())
```

rebuild and upgrade via helm

```sh
docker build -t quotegpt:latest .
helm upgrade quotegpt ./quotegpt
```

install the prometheus stack in a new namespace called monitoring
```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus prometheus-community/prometheus --namespace monitoring --create-namespace # do create-namespace here so we can initialize the monitoring namespace
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
helm install grafana grafana/grafana --namespace monitoring
```

Follow instructions for getting your grafana admin password `kubectl get secret --namespace monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo`. The username is `admin`. 

⚠️ if your prometheus node exporter is in a crash loop (I suspect it might have something to do with using docker desktop as minikube does not seem to have this problem) patch it with `kubectl patch ds prometheus-prometheus-node-exporter -n monitoring --type "json" -p '[{"op": "remove", "path" : "/spec/template/spec/containers/0/volumeMounts/2/mountPropagation"}]'` per this [workaround](https://github.com/prometheus-operator/kube-prometheus/discussions/790#discussioncomment-6896643) don't ask me why this is, I too get angry when shit doesnt work out of the box and requires hours and hours of debugging 😡

Add a `templates/servicemonitor.yaml` and upgrade the cluster with helm

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: api-service-monitor
  labels:
    release: prometheus-operator  # Must match the release name of your Prometheus Operator
spec:
  selector:
    matchLabels:
      app: api
  endpoints:
    - port: http
      path: /metrics
      interval: 15s
```

portforward prometheus and grafana 

```sh
kubectl port-forward svc/prometheus-server -n monitoring 9090:80
kubectl port-forward svc/grafana -n monitoring 3000:80
```

or expose the services

```sh
kubectl expose service prometheus-kube-prometheus-prometheus --type=NodePort --target-port=9090 --name=prometheus-node-port-service
kubectl expose service grafana --type=NodePort --target-port=3000 --name=grafana-node-port-service
```

Open a tab in the browser and go to localhost:9090 - this is prometheus's dashboard

Open another and go to localhost:3000 - this is grafana. log in with the credentials obtained a few steps ago.

