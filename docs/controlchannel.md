# Control Channel Flow & Operation

## Initiation a control channel session
- Client starts HTTP request against control channel endpoint
- Handler upgrades the request to a WebSocket connection
    * On error quit the HTTP handler
- Hand over WebSocket connection to controller