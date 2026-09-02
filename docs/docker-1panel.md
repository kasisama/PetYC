# 1Panel / 绑定宿主机目录部署

1Panel 等面板创建的 `./data`、`./config` 通常属于 root，镜像内进程用户是 `10001`，直接挂载会出现：

```text
open /config/.runtime-*.tmp: permission denied
```

面板编排请用下面这份（与仓库根目录 [`compose.1panel.yml`](../compose.1panel.yml) 相同）：启动时先用 root 修正目录权限，再降权运行 PetYC。

端口示例为 `23233`，可按机器情况改，但 `PORT`、`ports`、`healthcheck` 三处必须一致。

```yaml
services:
  petyc:
    image: ghcr.io/kasisama/petyc:latest
    container_name: petyc
    user: "0:0"
    init: true
    restart: unless-stopped
    stop_grace_period: 15s

    entrypoint:
      - /bin/sh
      - -c
      - |
        chown 10001:10001 /data /config
        chmod 700 /data /config
        exec su -p -s /bin/sh petyc -c 'exec /app/petyc'

    environment:
      LISTEN_ADDRESS: "0.0.0.0"
      PORT: "23233"
      QQPET_DATA_DIR: "/config"
      QQPET_WEB_SETUP: "1"
      TZ: "Asia/Shanghai"

    ports:
      - "127.0.0.1:23233:23233"

    volumes:
      - ./data:/data
      - ./config:/config

    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:23233/healthz"]
      interval: 30s
      timeout: 5s
      start_period: 20s
      retries: 3
```

容器健康后打开 `http://127.0.0.1:23233/admin`。OneBot 反向 WebSocket：

```text
ws://127.0.0.1:23233/v1/ws
```

不要把 `127.0.0.1` 改成 `0.0.0.0`。首次设密完成前，用 1Panel 网站反代到 `http://127.0.0.1:23233`，并启用 HTTPS。
