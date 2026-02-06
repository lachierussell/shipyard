# Test Server

- **Host**: ec2-user@54.206.54.73
- **OS**: FreeBSD 15.0 (arm64)
- **SSH**: `ssh -i ~/.ssh/AWS/parkcedar-default.pem ec2-user@54.206.54.73`

## Root Access

```sh
su -m root -c '<command>'   # Run single command
su -m root                  # Interactive shell
```

## Deploy from Source

```sh
# Package and copy
tar czf /tmp/shipyard.tar.gz --exclude='.git' --exclude='web/node_modules' .
scp -i ~/.ssh/AWS/parkcedar-default.pem /tmp/shipyard.tar.gz ec2-user@54.206.54.73:~

# Install on server
ssh -i ~/.ssh/AWS/parkcedar-default.pem ec2-user@54.206.54.73
su -m root
mkdir -p ~/shipyard && cd ~/shipyard && tar xzf ~/shipyard.tar.gz
cp -r ~/shipyard /usr/local/src/
/usr/local/src/shipyard/install.sh
```

## Useful Commands

```sh
service shipyard status          # Check status
service shipyard restart         # Restart
tail -f /var/log/shipyard/shipyard.log   # View logs
curl http://localhost:8443/health        # Test API
```

## Reset Server

```sh
pkill -9 shipyard
service nginx stop
rm -rf /usr/local/etc/shipyard /usr/local/bin/shipyard /usr/local/etc/rc.d/shipyard
rm -rf /usr/local/etc/nginx/sites-* /usr/local/etc/nginx/override.conf
rm -rf /var/log/shipyard /usr/local/www/*
```
