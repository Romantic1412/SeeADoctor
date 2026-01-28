[根目录](../CLAUDE.md) > **frontend**

---

# Frontend 模块文档

> 最后更新: 2026-01-27T23:51:59+08:00

## 变更记录 (Changelog)

- **2026-01-27**: 初始化前端模块文档

---

## 模块职责

`frontend` 模块是 QuickDoctor 的 Vue 3 前端界面，负责：

1. **用户界面**: 提供现代化的桌面应用界面（TailwindCSS + 渐变设计）
2. **状态管理**: 通过 Composables 管理登录状态、抢号任务、日志等
3. **Wails 通信**: 与 Go 后端的双向事件通信（方法调用 + 事件监听）
4. **配置管理**: 保存/加载抢号配置，快速修改日期进行下次抢号

---

## 入口与启动

### 主入口
- **文件**: `src/main.js`
- **功能**: 创建 Vue 应用实例，挂载到 `#app`
- **全局错误处理**: 捕获 Vue 错误并弹窗提示

### 根组件
- **文件**: `src/App.vue`
- **功能**:
  - 初始化 Composables（日志、登录、抢号监听器）
  - 管理页面导航（Dashboard / ConfigPanel / TaskMonitor）
  - 加载用户配置状态

---

## 对外接口

### 页面组件（Views）

| 组件 | 路由名称 | 功能 |
|------|---------|------|
| `Dashboard.vue` | `dashboard` | 主控台：登录、抢号控制、配置快览 |
| `ConfigPanel.vue` | `config` | 配置面板：医院/科室/医生选择、抢号参数设置 |
| `TaskMonitor.vue` | `logs` | 日志监控：实时日志显示与导出 |

### UI 组件（Components）

| 组件 | 功能 |
|------|------|
| `AppShell.vue` | 应用外壳：侧边栏导航、顶栏状态 |
| `GlassCard.vue` | 玻璃态卡片容器 |
| `NeonButton.vue` | 霓虹风格按钮 |
| `StatusBadge.vue` | 状态徽章（成功/失败/警告） |
| `LogViewer.vue` | 日志查看器（虚拟滚动） |
| `Combobox.vue` | 下拉选择器（支持搜索） |

### Composables（逻辑复用）

| 文件 | 功能 |
|------|------|
| `useAuth.js` | 登录状态管理、扫码登录流程 |
| `useGrabTask.js` | 抢号任务管理、状态监听 |
| `useHospitalData.js` | 医院/科室/医生数据查询 |
| `useConfigManager.js` | 配置保存/加载（核心功能） |
| `useLogger.js` | 日志管理与导出 |

---

## 关键依赖与配置

### 依赖清单

```json
// package.json
{
  "dependencies": {
    "vue": "^3.2.37"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^3.0.3",
    "vite": "^3.0.7",
    "tailwindcss": "^3.4.19",
    "autoprefixer": "^10.4.23",
    "postcss": "^8.5.6"
  }
}
```

### 构建配置

- **Vite**: `vite.config.js` - 开发服务器与构建工具
- **TailwindCSS**: `tailwind.config.js` - 样式框架配置
- **PostCSS**: `postcss.config.js` - CSS 后处理

### Wails 集成

- **绑定文件**: `wailsjs/go/main/App.js` - 自动生成的 Go 方法绑定
- **类型定义**: `wailsjs/go/models.ts` - TypeScript 类型定义
- **运行时**: `wailsjs/runtime/runtime.js` - Wails 运行时工具

---

## 数据模型

### 用户状态 (UserState)

```javascript
// 来自 useAuth.js
const userState = ref({
  city_id: '',
  unit_id: null,
  unit_name: null,
  dep_id: null,
  dep_name: null,
  doctor_id: null,
  doctor_name: null,
  member_id: null,
  target_dates: [],
  preferred_hours: [],
  time_types: [],
  schedule_id: null,
  proxy_submit_enabled: false
})
```

### 日志条目 (LogEntry)

```javascript
// 来自 useLogger.js
{
  time: '2026-01-27 23:51:59',
  level: 'info' | 'success' | 'warn' | 'error',
  message: '登录成功'
}
```

### 抓取配置 (GrabConfig)

```javascript
// 传递给 Go 后端的配置
{
  unit_id: '123',
  unit_name: '深圳市人民医院',
  dep_id: '456',
  dep_name: '心内科',
  doctor_id: '789',
  doctor_name: '张医生',
  member_id: '001',
  target_dates: ['2026-01-28'],
  time_types: ['am'],
  preferred_hours: ['08:00'],
  proxy_submit_enabled: true
}
```

---

## 测试与质量

### 测试覆盖

**当前状态**: 无自动化测试

**建议补充**:
1. **Composables 单元测试**
   - 使用 Vitest + @vue/test-utils
   - 测试 `useAuth` 登录流程
   - 测试 `useConfigManager` 配置加载/保存

2. **组件测试**
   - 测试 `Combobox` 搜索与选择
   - 测试 `LogViewer` 虚拟滚动
   - 测试 `Dashboard` 按钮交互

3. **E2E 测试**
   - 使用 Playwright 或 Cypress
   - 测试完整的抢号流程

### 代码质量工具

建议配置:
- **ESLint**: JavaScript 代码规范检查
- **Prettier**: 代码格式化
- **TypeScript**: 逐步引入类型安全（当前为纯 JS）

---

## 常见问题 (FAQ)

### Q1: 如何调用 Go 后端方法？

**A**: 使用自动生成的绑定：
```javascript
import { GetCities, StartGrab } from '../wailsjs/go/main/App'

// 调用 Go 方法
const cities = await GetCities()
await StartGrab(config)
```

### Q2: 如何监听 Go 后端事件？

**A**: 使用 Wails 运行时：
```javascript
import { EventsOn } from '../wailsjs/runtime/runtime'

EventsOn('log-message', (data) => {
  console.log(data.level, data.message)
})
```

### Q3: 配置管理如何工作？

**A**: 见 `docs/CONFIG_MANAGEMENT_GUIDE.md`。核心流程：
1. 在 ConfigPanel 配置参数
2. 点击"保存配置"调用 `SaveGrabConfig()`
3. 下次打开点击"加载配置"恢复设置
4. 只需修改日期即可快速抢号

### Q4: 如何自定义 UI 主题？

**A**: 修改 `tailwind.config.js` 中的 `theme.extend`：
```javascript
colors: {
  primary: '#3B82F6',
  secondary: '#8B5CF6',
  // ...
}
```

### Q5: 开发时前端独立运行吗？

**A**: 可以（`npm run dev`），但无法调用 Go 方法。建议使用 `wails dev` 获得完整功能。

---

## 相关文件清单

### 核心代码
- `src/main.js` - Vue 应用入口
- `src/App.vue` - 根组件
- `src/style.css` - 全局样式

### 视图组件
- `src/components/views/Dashboard.vue` - 主控台
- `src/components/views/ConfigPanel.vue` - 配置面板
- `src/components/views/TaskMonitor.vue` - 日志监控

### UI 组件
- `src/components/ui/AppShell.vue` - 应用外壳
- `src/components/ui/GlassCard.vue` - 玻璃卡片
- `src/components/ui/NeonButton.vue` - 霓虹按钮
- `src/components/ui/StatusBadge.vue` - 状态徽章
- `src/components/ui/LogViewer.vue` - 日志查看器
- `src/components/ui/Combobox.vue` - 下拉选择器

### Composables
- `src/composables/useAuth.js` - 登录管理
- `src/composables/useGrabTask.js` - 抢号任务
- `src/composables/useHospitalData.js` - 医院数据
- `src/composables/useConfigManager.js` - 配置管理
- `src/composables/useLogger.js` - 日志管理

### 构建配置
- `vite.config.js` - Vite 配置
- `tailwind.config.js` - TailwindCSS 配置
- `postcss.config.js` - PostCSS 配置
- `package.json` - 依赖管理

---

## 架构建议

### 优化方向
1. **引入 TypeScript**: 提高类型安全，减少运行时错误
2. **状态持久化**: 使用 localStorage 或 IndexedDB 缓存部分状态
3. **组件拆分**: `ConfigPanel.vue` 较长，可拆分为子组件
4. **虚拟滚动优化**: `LogViewer` 在日志量大时可能卡顿，考虑使用 `vue-virtual-scroller`

### 扩展性
- **国际化**: 可引入 `vue-i18n` 支持多语言
- **主题切换**: 支持明暗主题切换
- **插件系统**: 支持用户自定义脚本或插件
