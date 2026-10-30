# macOS 公证准备清单（无密钥）

工程包可 `darwin:package` 产出；**公证与分发签名需 Apple Developer 账号**，本文件只列步骤备忘，不存放证书、App 专用密码或 Team ID 机密值。

## 前置

- [ ] 有效 Apple Developer Program 会员
- [ ] 本机 Xcode / `xcrun notarytool` 可用
- [ ] Developer ID Application 证书已装入钥匙串（名称记入内部运维，勿提交仓库）
- [ ] App 专用密码或 API Key（App Store Connect）仅存本机钥匙串 / CI secret，**勿写入 git**

## 打包

- [ ] 仓库根按 CI 同款打 macOS 包（如 `wails3` / `darwin:package`）
- [ ] 确认产物为 `.app`（或 zip 内含 `.app`），bundle id 与证书一致
- [ ] 对 `.app` 做 Developer ID 签名（hardened runtime）；检查 entitlements 是否与 Wails/Seatbelt 需求冲突

## 公证

- [ ] `xcrun notarytool submit <归档.zip|dmg> --wait`（凭证走钥匙串 profile 或 CI secret 注入）
- [ ] 成功后对 `.app` / `.dmg` 执行 `xcrun stapler staple`
- [ ] 干净机（或另一用户）打开：Gatekeeper 无「无法验证开发者」阻断

## 分发（可选）

- [ ] 上传公证后的 zip/dmg 至内部分发渠道
- [ ] 版本号 / commit 写入发布说明；更新 [DEV-B](../docs/DEV-B.md) 或发布记录
- [ ] 轮换或撤销曾泄露的 App 专用密码

## 明确不做

- 不把 `.p12`、App 专用密码、Issuer ID、私钥提交进本仓库
- 未公证前不以「安装签名已完成」关 P1-B-06
