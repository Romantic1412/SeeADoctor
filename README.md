# QuickDoctor（Go + Wails）

基于 Go + Wails 的 Windows 挂号助手，支持微信扫码登录、号源查询与自动提交。

## ✨ 功能概览
- 微信扫码登录（TLS 指纹客户端）
- 城市/医院/科室/医生联动查询
- 号源查询与自动提交
- 前端日志与状态提示
- 提交过频自动退避（可选代理提交）
- **配置管理**：一键保存/加载配置，快速修改日期进行下次抢号（详见 [配置管理使用指南](docs/CONFIG_MANAGEMENT_GUIDE.md)）

## 🧰 开发环境
- Go 1.23（以 `go.mod` 为准）
- Node.js 20+（用于前端构建）
- Wails CLI v2

安装 Wails CLI：
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
```

## 🚀 本地开发
```bash
wails dev
```

## 📦 打包构建（Windows）
```bash
wails build
```

构建产物位于 `build/bin/`。

## 📂 配置与数据
- `config/cities.json`：城市列表
- `config/user_state.json`：UI 状态（本地）
- `config/cookies.json`：登录 Cookie（本地）
- `logs/`：运行日志与提交响应

## 📚 迁移契约
接口与数据契约见 `docs/contract.md`，用于前后端对齐与验证。

## ⚠️ 免责声明
本软件仅供技术研究与学习使用，请勿用于非法用途或商业获利。使用本软件产生的任何后果由用户自行承担。
