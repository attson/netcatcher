#!/usr/bin/env bash

version=$NIAR_VERSION
if [ "$version" = "" ]; then
  echo "please provide NIAR_VERSION env"
  exit 1
fi

trimVerion=${version#v}

echo "
get execute binary file...
  - wget "https://github.com/attson/niar/releases/download/${version}/niar_${trimVerion}_darwin_amd64.tar.gz"
  - mkdir "/usr/local/bin/niar_${trimVerion}_darwin_amd64"
  - tar -zxf "niar_${trimVerion}_darwin_amd64.tar.gz" -C "/usr/local/bin/niar_${trimVerion}_darwin_amd64"
  - chmod +x "/usr/local/bin/niar_${trimVerion}_darwin_amd64/niar"
"

wget "https://github.com/attson/niar/releases/download/${version}/niar_${trimVerion}_darwin_amd64.tar.gz"
mkdir "/usr/local/bin/niar_${trimVerion}_darwin_amd64"
tar -zxf "niar_${trimVerion}_darwin_amd64.tar.gz" -C "/usr/local/bin/niar_${trimVerion}_darwin_amd64"
chmod +x "/usr/local/bin/niar_${trimVerion}_darwin_amd64/niar"
content=$(cat <<-END
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>UserName</key>
    <string>root</string>
    <key>Label</key>
    <string>com.attson.niar</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ProcessType</key>
    <string>Background</string>
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/com.attson.niar.log</string>
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/com.attson.niar.log</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/niar_${trimVerion}_darwin_amd64/niar</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/usr/local/bin/niar_${trimVerion}_darwin_amd64</string>
  </dict>
</plist>
END
)

echo "create launchctl plist to /Library/LaunchDaemons/com.attson.niar.plist with root. maybe need ask for password"

sudo bash -c "echo \"$content\" > /Library/LaunchDaemons/com.attson.niar.plist"

sudo launchctl unload /Library/LaunchDaemons/com.attson.niar.plist || true
sudo launchctl load /Library/LaunchDaemons/com.attson.niar.plist

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

echo "$config" > "/usr/local/bin/niar_${trimVerion}_darwin_amd64/config.json"

echo ""
echo "----------- install success. enjoy it! ----------"
echo ""
echo "default config: don't forget update"
echo $config
echo "edit config: vim /usr/local/bin/niar_${trimVerion}_darwin_amd64/config.json"
echo "start cmd: sudo launchctl start com.attson.niar"
echo "stop cmd: sudo launchctl stop com.attson.niar"