# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A minimal Go HTTP server, containerized with Docker, deployed to a local
Kubernetes cluster via `kind`. It exists to practice core Kubernetes
concepts (Pod, Deployment, Service) rather than to be a real application.

- `server.go` — single-file `package main`, no `go.mod`, no external
  dependencies (stdlib `net/http` only). Listens on `:80` and returns
  `"Hello Full Cycle!"` for every route.
- `Dockerfile` — builds the server inside `golang:1.15-alpine` and runs it.
- `k8s/` — Kubernetes manifests: `pod.yaml`, `deployment.yaml`,
  `service.yaml`.
- `docs/` — the project's documentation, written in Portuguese. Start at
  `docs/README.md` for the full build → kind → deploy → access workflow;
  the other files each cover one concept in depth (`kubernetes.md`,
  `pods.md`, `deployments.md`, `services.md`, `kind.md`, `kubectl.md`,
  `golang.md`).

## Commands

There is no `go.mod`, Makefile, test suite, or linter configured. Building
and running only happens through Docker/Kubernetes:

```bash
# Build the image
docker build -t thomasmoraes02/hello-go .

# Run it locally to sanity-check before deploying
docker run --rm -p 80:80 thomasmoraes02/hello-go

# Create the local cluster
kind create cluster --name fullcycle

# Load the locally-built image into the kind cluster (kind nodes can't see
# the host's Docker images otherwise — see docs/kind.md)
kind load docker-image thomasmoraes02/hello-go:latest --name fullcycle

# Apply manifests
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# Reach the app (kind does not provision a real LoadBalancer)
kubectl port-forward svc/goserver-service 8080:80
```

## Architecture notes

- The Deployment (`k8s/deployment.yaml`) runs 10 replicas of the
  `goserver` Pod template, selected by the `app: goserver` label.
- The Service (`k8s/service.yaml`) is `type: LoadBalancer` and routes to
  Pods via the same `app: goserver` selector; on `kind` it stays
  `EXTERNAL-IP: <pending>`, so use `kubectl port-forward` to access it
  locally instead.
- Known port mismatch across the stack: `server.go` listens on `:80`, but
  `deployment.yaml` declares `containerPort: 8080` and `service.yaml` sets
  `targetPort: 8000`. `containerPort` is documentation-only so the
  Deployment still runs fine, but the Service's `targetPort` needs to be
  `80` to actually reach the process — see `docs/services.md` for the full
  explanation. Be aware of this when touching the manifests or debugging
  connectivity.
- Images are tagged `thomasmoraes02/hello-go:latest`; since the tag is
  `latest`, the default `imagePullPolicy: Always` means the cluster tries
  to pull from a registry unless the image was explicitly loaded via
  `kind load docker-image` (see `docs/kind.md`).

## Repository context

This directory lives inside the Homebrew (`/opt/homebrew`) prefix on disk
but is excluded from that repository via `.gitignore`
(`var/www/projects/fullcycle/3.0/kubernetes`) — it is not part of the
Homebrew/brew project and Homebrew's `AGENTS.md` conventions (Sorbet,
RuboCop, etc.) do not apply here.
