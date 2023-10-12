#!/usr/bin/env bash

version=$NETCATCHER_VERSION
if [ "$version" = "" ]; then
  echo "please provide NETCATCHER_VERSION env"
  exit 1
fi

os=$NETCATCHER_OS
if [ "$os" = "" ]; then
  echo "please provide NETCATCHER_OS env"
  exit 1
fi

trimVerion=${version#v}

echo "
get execute binary file...
  - wget "https://github.com/attson/netcatcher/releases/download/${version}/netcatcher_${trimVerion}_${os}.tar.gz"
  - mkdir "/usr/local/bin/netcatcher_${trimVerion}_${os}"
  - tar -zxf "netcatcher_${trimVerion}_${os}.tar.gz" -C "/usr/local/bin/netcatcher_${trimVerion}_${os}"
  - chmod +x "/usr/local/bin/netcatcher_${trimVerion}_${os}/netcaptcher"
"

wget "https://github.com/attson/netcatcher/releases/download/${version}/netcatcher_${trimVerion}_${os}.tar.gz"
mkdir "/usr/local/bin/netcatcher_${trimVerion}_${os}"
tar -zxf "netcatcher_${trimVerion}_${os}.tar.gz" -C "/usr/local/bin/netcatcher_${trimVerion}_${os}"
chmod +x "/usr/local/bin/netcatcher_${trimVerion}_${os}/netcatcher"
content=$(cat <<-END
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>UserName</key>
    <string>root</string>
    <key>Label</key>
    <string>com.attson.netcatcher</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ProcessType</key>
    <string>Background</string>
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/com.attson.netcatcher.log</string>
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/com.attson.netcatcher.log</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/netcatcher_${trimVerion}_${os}/netcatcher</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/usr/local/bin/netcatcher_${trimVerion}_${os}</string>
  </dict>
</plist>
END
)

echo "create launchctl plist to /Library/LaunchDaemons/com.attson.netcatcher.plist with root. maybe need ask for password"

sudo bash -c "echo \"$content\" > /Library/LaunchDaemons/com.attson.netcatcher.plist"

sudo launchctl unload /Library/LaunchDaemons/com.attson.netcaptcher.plist || true
sudo launchctl load /Library/LaunchDaemons/com.attson.netcaptcher.plist

if [ ! -f "/usr/local/bin/netcatcher_${trimVerion}_${os}/config.json" ]; then
  config=$(cat <<-END
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
  END
  )
  echo "$config" > "/usr/local/bin/netcatcher_${trimVerion}_${os}/config.json"
fi

echo ""
echo "----------- install success. enjoy it! ----------"
echo ""
echo "default config: don't forget update"
cat "/usr/local/bin/netcatcher_${trimVerion}_${os}/config.json"
echo "edit config: vim /usr/local/bin/netcatcher_${trimVerion}_${os}/config.json"
echo "[notice] com.attson.netcatcher runAtLoad..."
echo "start cmd: sudo launchctl load /Library/LaunchDaemons/com.attson.netcatcher.plist"
echo "stop cmd: sudo launchctl unload /Library/LaunchDaemons/com.attson.netcatcher.plist"