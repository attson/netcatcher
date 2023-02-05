# Network Interface Automatic Route

A small tool help you add static route to specified network interface. When the specified network interface is
connected, it adds routes.

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

## quick install

**edit route need root permission. so you should run niar with root**

```bash
sudo ./niar
```

### macos with launchctl

```
curl -s https://raw.githubusercontent.com/attson/niar/main/install/darwin-amd64.sh | NIAR_VERSION=v0.0.4 bash
```

## run log

```bash
$ tail -f /usr/local/var/log/com.attson.niar.log
2023/02/05 17:01:32 niar started...
2023/02/05 17:01:32 ppp0: [info] interface status is connected
add host 140.82.113.3: gateway 192.168.199.51
2023/02/05 17:01:32 ppp0: [debug] add route github.com -> 140.82.113.3 @ 192.168.199.51
add host 192.168.188.11: gateway 192.168.199.51
2023/02/05 17:01:32 ppp0: [debug] add route 192.168.188.11 -> 192.168.188.11 @ 192.168.199.51
add net 192.168.188.0: gateway 192.168.199.51
2023/02/05 17:01:32 ppp0: [debug] add route 192.168.188.0/24 -> 192.168.188.0/24 @ 192.168.199.5
```

## tested on

- [x] macos