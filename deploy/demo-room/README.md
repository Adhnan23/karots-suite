# Web Demo Room (demo.karots.lk)

The Demo Room on the OVH VPS, behind nginx-proxy-manager like the other sites there. Visitors still need the
hourly code (the lock page offers WhatsApp and the phone number to ask for one); unlocked visitors share the one
running Demo Room.

## Update it

```sh
make demo                      # dist/demo/karots-suite-demo-room
make demo-room-deploy          # copies it up and restarts the container (DEMO_HOST, default the VPS)
```

## First time only

1. Cloudflare → karots.lk → DNS: `A demo → 144.217.83.102`, proxied.
2. nginx-proxy-manager (`ssh -L 8181:127.0.0.1:81 144.217.83.102`, then http://localhost:8181) → Proxy Hosts →
   Add: domain `demo.karots.lk`, scheme `http`, forward host `karots-demo-room`, port `3200`, Websockets on,
   Block common exploits on; SSL: request a new certificate, Force SSL on.
