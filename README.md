# Data Availability Server for Nubit DA

## Modify the configuration
```
cd config
// Make a copy of the sample file
cp config.json.example config.json
// Replace your own configuration for config.json

```

## Build

`cd cmd && go build -o nubit-server`

## Configuration
```
{
    "rpc-port": 9876,                                              // gEPC server port
    "nubit": {  
        "gas-price": 0.01,
        "gas-multiplier": 1.01,
        "rpc": "http://localhost:26658",                           // nubit node rpc
        "auth-token": "token",                                     // nubit node auth token
        "namespace-id": "nuport",
        "validator-config": {
            "tendermint-rpc":"http://localhost:26657",             // Tendermint rpc
            "eth-rpc": ${ETH_RPC},                                 // eth rpc
            "nuport":"0x3101F6BB7f65c5BfA973391E6A4E4eaC42ea91fA", // nuport contract address
            "verify": "0x031dDB92361BbC28Cd7Be3a6F102c0F0Db230aE3" // nuport verify contract address
        }
    }
}
```


## Run app

```
./nubit-sever
```
