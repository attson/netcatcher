# Network Interface Automatic Route

A small tool help you add static route to specified network interface. When the specified network interface is connected, it adds routes.

It is useful when work with multiple network interfaces. (or with vpn)


## config

support domain, fixed ip, ip mask

```json
{
  "interfaces": [
    {
      "name": "ppp0",
      "routes": [
        "github.com",
        "192.168.188.11",
        "192.168.188.0/24"
      ]
    }
  ]
}
```

## tested on

- [x] macos