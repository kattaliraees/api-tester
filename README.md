# Meeting PoC API

This is the backend API for the Meeting Proof of Concept, built with Go and running in Docker.

## Overview

The API provides endpoints for attendance registration and GPS tracking:

- `/update`: For attendance events (from ESP32 or mobile app)
- `/gps`: For GPS location updates (from mobile app)

## Endpoints

### GET /update?id=<uuid>&value=<bool>

Registers attendance for a device.

- `id`: Device UUID (e.g., beacon UUID)
- `value`: Boolean flag (true for attendance)

Example: `GET /update?id=550e8400-e29b-41d4-a716-446655440000&value=true`

Response: `Device 550e8400-e29b-41d4-a716-446655440000 set to true`

### GET /gps?id=<device_id>&lat=<latitude>&lon=<longitude>

Updates GPS location for a device.

- `id`: Device identifier
- `lat`: Latitude (float)
- `lon`: Longitude (float)

Example: `GET /gps?id=device-1&lat=37.7749&lon=-122.4194`

Response: `GPS updated for device-1: 37.774900, -122.419400`

## Building and Running

### Prerequisites
- Docker installed

### Build the Docker Image
```bash
docker build -t esp32-api .
```

### Run the Container
```bash
docker run -p 8080:8080 esp32-api
```

The API will be available at `http://localhost:8080`.

## Logs

The API logs all requests to the console, including attendance registrations and GPS updates.