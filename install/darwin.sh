#!/usr/bin/env bash
set -e

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
get execute binary file ... some cmd with root. maybe need ask for password
  - curl -LO "https://github.com/attson/netcatcher/releases/download/${version}/netcatcher_${trimVerion}_${os}.tar.gz"
  - sudo mkdir "/usr/local/bin/netcatcher_${trimVerion}_${os}"
  - sudo tar -zxf "netcatcher_${trimVerion}_${os}.tar.gz" -C "/usr/local/bin/netcatcher_${trimVerion}_${os}"
  - sudo chmod +x "/usr/local/bin/netcatcher_${trimVerion}_${os}/netcatcher"
"

curl -LO "https://github.com/attson/netcatcher/releases/download/${version}/netcatcher_${trimVerion}_${os}.tar.gz"
sudo mkdir -p "/usr/local/bin/netcatcher_${trimVerion}_${os}"
sudo tar -zxf "netcatcher_${trimVerion}_${os}.tar.gz" -C "/usr/local/bin/netcatcher_${trimVerion}_${os}"
sudo chmod +x "/usr/local/bin/netcatcher_${trimVerion}_${os}/netcatcher"

echo "create launchctl plist to /Library/LaunchDaemons/com.attson.netcatcher.plist"

sudo tee /Library/LaunchDaemons/com.attson.netcatcher.plist << EOF > /dev/null
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
EOF

sudo launchctl unload /Library/LaunchDaemons/com.attson.netcatcher.plist || true
sudo launchctl load /Library/LaunchDaemons/com.attson.netcatcher.plist

if [ ! -f "/usr/local/bin/netcatcher_${trimVerion}_${os}/config.json" ]; then
  sudo tee "/usr/local/bin/netcatcher_${trimVerion}_${os}/config.json" << 'EOF' > /dev/null
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
EOF
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