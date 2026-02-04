# PDF Translator Map-Reduce

PDF Translator Map-Reduce is a distributed PDF translation system designed to run on Kubernetes (Minikube) using RPC.
---

## Prerequisites

- Docker
- Minikube
- kubectl
- Git
- OpenAI API key

---

## Initial Setup (Run Once)

Create the required Kubernetes resources.

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/pvc.yaml
```

Create the OpenAI API key secret (replace with your actual key).

```bash
kubectl create secret generic openai-secret -n mr \
  --from-literal=OPENAI_API_KEY='<your_openai_api_key>'
```

Start the loader pod used to manage input and output files.

```bash
kubectl apply -f k8s/loader.yaml
```

---

## Build and Run Workflow

Repeat this workflow whenever the source code changes.

### Build Docker Image

Build a new docker image and load the image into minikube.

```bash
export NEW_TAG=0.4
docker build -t my-mr:$NEW_TAG .
minikube image load my-mr:$NEW_TAG
```

Also, you have to change the tags in the master, worker yaml files.
Update the `image:` field in the following files to match the new tag:

- `k8s/master.yaml`
- `k8s/worker.yaml`

Example: `my-mr:0.4`

### Redeploy Application

Delete existing resources.

```bash
kubectl delete deployment mr-master -n mr --ignore-not-found=true
kubectl delete job mr-worker -n mr --ignore-not-found=true
```

Copy the input PDF into the shared volume.

```bash
kubectl cp <path_to_your_pdf> mr-loader:/data/input.pdf -n mr
```

Deploy the master and worker.

```bash
kubectl apply -f k8s/master.yaml
kubectl apply -f k8s/worker.yaml
```
---

## Check Results

Wait for the worker job to complete.

```bash
kubectl get job mr-worker -n mr --watch
```

List files in the shared volume.

```bash
kubectl exec mr-loader -n mr -- ls -l /data
```

Copy the translated PDF to the local machine.

```bash
kubectl cp mr-loader:/data/translated_input.pdf ./translated.pdf -n mr
```