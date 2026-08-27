# PetYC 更新系统

## 支持范围

- Windows amd64 便携版：支持检查、下载、签名校验、自动替换和重启。
- Linux amd64 独立二进制：支持相同流程。
- Docker、systemd、非 amd64、开发构建或程序目录不可写：仅提示新版本并引导到 GitHub Release。

更新接口位于管理员会话保护下：

- `GET /api/admin/updates/check`：使用 24 小时缓存检查稳定版；追加 `?force=1` 强制刷新。
- `GET /api/admin/updates/status`：读取下载、校验和重启状态。
- `POST /api/admin/updates/install`：启动唯一的后台更新任务。

`GET /healthz` 是更新辅助进程使用的无鉴权存活探针，只返回 `status` 和构建版本。

## 发布密钥

公钥位于 `updater/public_key.go`，私钥不得提交到仓库。GitHub 仓库需要配置：

```text
UPDATE_SIGNING_PRIVATE_KEY
```

其值是 base64 编码的 64 字节 Ed25519 私钥。可以使用项目内工具生成密钥：

```powershell
go run ./cmd/signmanifest -generate `
  -public-output update-public.key `
  -private-output update-private.key
```

生成后应把公钥写入 `updater.DefaultPublicKey`，把私钥存入 GitHub Actions Secret，并把本地私钥保存在受限目录。

## 发布产物

推送 `vX.Y.Z` 标签后，Release 工作流生成：

```text
petyc_X.Y.Z_windows_amd64.exe
petyc_X.Y.Z_linux_amd64
update-manifest.json
update-manifest.json.sig
```

签名覆盖 `update-manifest.json` 的原始字节；清单内部的 SHA-256 和大小用于验证下载后的平台二进制。

## 恢复

更新辅助进程在旧进程退出后，将旧程序、`pet_game.db` 以及存在时的 `-wal`、`-shm` 文件保存到程序目录下：

```text
.petyc-backups/YYYYMMDD-HHMMSS/
```

新版本在 60 秒内未以期望版本通过 `/healthz` 时，辅助进程会终止新进程、恢复程序和数据库并重新启动旧版本。发布包含数据库结构变更的版本前，仍必须验证旧版本能够读取迁移后的数据库；破坏性迁移应拆成独立的后置清理版本。

