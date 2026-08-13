import {
  IconBook2,
  IconBrandQq,
  IconLayoutDashboard,
  IconMap2,
  IconSettings,
  IconUsers,
  IconWorld,
} from '@tabler/icons-vue'

export const adminNavItems = [
  { name: 'dashboard', label: '运营总览', shortLabel: '总览', icon: IconLayoutDashboard },
  { name: 'players', label: '玩家管理', shortLabel: '玩家', icon: IconUsers },
  { name: 'gameplay', label: '玩法运营', shortLabel: '玩法', icon: IconMap2 },
  { name: 'communities', label: '社群运营', shortLabel: '社群', icon: IconWorld },
  { name: 'content', label: '内容配置', shortLabel: '内容', icon: IconBook2 },
  { name: 'platforms', label: '平台状态', shortLabel: '平台', icon: IconBrandQq },
  { name: 'system', label: '系统设置', shortLabel: '系统', icon: IconSettings },
] as const
