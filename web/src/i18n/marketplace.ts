export const marketplace: Record<string, string> = {
  '语法高亮加载失败，点击重试':
    'Syntax highlighting failed to load. Click to retry.',
  '正在载入编辑器…': 'Loading editor…',
  技能元数据: 'Skill metadata',
  文件工作区: 'File workspace',
  收起文件目录: 'Hide explorer',
  展开文件目录: 'Show explorer',
  上传文件: 'Upload files',
  工作区全屏: 'Fullscreen workspace',
  还原工作区: 'Restore workspace',
  已打开文件: 'Open files',
  '切换到 {path}': 'Switch to {path}',
  '关闭标签 {path}': 'Close tab {path}',
  '文件夹 {path}': 'Folder {path}',
  '{count} 个文件': '{count} files',
  查看方式: 'View mode',
  预览: 'Preview',
  分栏: 'Split',
  自动换行: 'Word wrap',
  下载文件: 'Download file',
  文件预览: 'File preview',
  未保存: 'Unsaved',
  图片: 'Image',
  仅预览技能内的图片: 'Only images in this skill are previewed',
  '图片无法预览，请下载查看。':
    'Unable to preview this image. Download to view it.',
  '行 {line}，列 {column}': 'Ln {line}, Col {column}',
  全屏编辑: 'Edit fullscreen',
  全屏: 'Fullscreen',
  退出全屏: 'Exit fullscreen',
  '同步了 {mcp} 个 MCP、{skills} 个技能；失败 {failed} 项':
    'Synced {mcp} MCP entries and {skills} skills; {failed} failed',
  已加入工具池: 'In tool pool',
  '安装到工具池 {value1}': 'Install into tool pool {value1}',
  移出工具池: 'Remove from pool',
  '移出工具池 {value1}': 'Remove {value1} from tool pool',
  '请先填写 SKILL.md，再保存技能。':
    'Fill in SKILL.md before saving the skill.',
  编辑技能: 'Edit skill',
  基本信息: 'Basic information',
  'GitHub 导入与同步': 'GitHub import and sync',
  '编辑已发布文件时，保存不会重新拉取 GitHub。':
    'Saving edited files does not fetch GitHub again.',
  编辑技能文件: 'Edit skill files',
  '从 GitHub 重新同步': 'Sync from GitHub',
  '同步会用 GitHub 目录替换已发布的全部文件，本地编辑不会一并保存。':
    'Sync replaces all published files with the GitHub directory. Local edits are not included.',
  '文件管理：上传与移除': 'Manage files: upload and remove',
  技能文件: 'Skill files',
  新建文本文件: 'New text file',
  新文件路径: 'New file path',
  创建文件: 'Create file',
  '路径无效、文件已存在或文件数量已达上限。':
    'Invalid path, file already exists, or file limit reached.',
  '{path} 内容': '{path} content',
  '此文件不支持文本编辑，可在文件管理中上传替换。':
    'This file cannot be edited as text. Upload a replacement in file management.',
  '正在读取文件…': 'Loading file…',
  '文件内容不可用，请关闭后重试。':
    'File content is unavailable. Close and try again.',
  '在这里编写 AI 应遵循的步骤与要求，支持 Markdown。':
    'Write the steps and requirements for the AI. Markdown is supported.',
  '附件目录（可选）': 'Attachment directory (optional)',
  添加附件: 'Add attachments',
  '附件路径无效；使用相对目录，SKILL.md 请在上方编辑。':
    'Invalid attachment path. Use a relative directory and edit SKILL.md above.',
  '单个附件最多 8 MiB。': 'Each attachment must be no larger than 8 MiB.',
  '技能最多 32 个文件，总大小最多 32 MiB。':
    'A skill may contain up to 32 files and 32 MiB in total.',
  '附件读取失败，请重试。': 'Could not read the attachment. Please retry.',
  '附件保留原始字节，不做图片压缩。每个文件最多 8 MiB，含 SKILL.md 共 32 个文件、32 MiB；较大附件需配置对象存储。':
    'Attachments retain their original bytes without image compression. Up to 8 MiB per file, 32 files and 32 MiB including SKILL.md. Larger attachments require object storage configuration.',
  '正在读取附件…': 'Reading attachments…',
  清除附件错误: 'Clear attachment error',
  可执行: 'Executable',
  '可执行 {path}': 'Executable {path}',
  '移除 {path}': 'Remove {path}',
  移除: 'Remove',
  '未提供 HTTP 端点的 MCP 暂不可加入装备组。':
    'MCP entries without an HTTP endpoint cannot be added yet.',
  一键同步全部: 'Sync all',
  '同步完成：成功 {success}，失败 {failed}':
    'Sync complete: {success} succeeded, {failed} failed',
  '同步失败，请重试': 'Sync failed, please retry',
  市场管理: 'Marketplace management',
  '创建技能、组合装备组，或从 GitHub 同步开源技能。':
    'Create skills, compose bundles, or sync open-source skills from GitHub.',
  创建技能: 'Create skill',
  创建装备组: 'Create bundle',
  编辑市场条目: 'Edit marketplace item',
  '精选 GitHub 技能': 'Selected GitHub skills',
  保存技能: 'Save skill',
  保存装备组: 'Save bundle',
  名称: 'Name',
  标识: 'Identifier',
  描述: 'Description',
  '3–32 位字母、数字、短横线或下划线，用于安装命令。':
    '3–32 letters, digits, hyphens or underscores, used in install commands.',
  技能来源: 'Skill source',
  自定义内容: 'Custom content',
  'SKILL.md 内容': 'SKILL.md content',
  '使用 Markdown 编写技能说明；编辑时保留已有附属文件。':
    'Write skill instructions in Markdown. Existing supporting files are preserved when editing.',
  'GitHub 仓库': 'GitHub repository',
  技能目录: 'Skill directory',
  '填写包含 SKILL.md 的目录；同步失败时保留原条目。':
    'Enter the directory containing SKILL.md. Failed syncs preserve the existing entry.',
  '选择组件（最多 32 项）': 'Select components (up to 32)',
  '先创建技能或同步 MCP，再组合装备组。':
    'Create skills or sync MCP entries before composing a bundle.',
  '从维护者仓库同步完整技能目录；可重复同步。源码与许可请查看原仓库。':
    'Sync complete skill directories from maintainer repositories. Sync again to refresh. See each repository for source and license.',
  已同步: 'Synced',
  同步: 'Sync',
  '同步 {value1}': 'Sync {value1}',
  已保存: 'Saved',
  '系统排障：定位根因、验证假设。': 'Find root causes and verify hypotheses.',
  '先写失败测试，再实现与重构。':
    'Write failing tests, then implement and refactor.',
  '构建有辨识度的前端界面。': 'Build distinctive frontend interfaces.',
  '技能保存工作方法，装备组组合 MCP 与技能。保存后可在公共市场查看并安装。':
    'Skills capture workflows; bundles combine MCP tools and skills. Saved entries are available in the public marketplace.',
  类型筛选: 'Filter by type',
  搜索市场: 'Search marketplace',
  '搜索名称、标识或描述': 'Search names, identifiers or descriptions',
  组合安装: 'Bundle installation',
  网关托管: 'Gateway hosted',
};
