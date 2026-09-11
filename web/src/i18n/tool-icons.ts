export const toolIcons: Record<string, string> = {
  '请选择 PNG、JPEG、WebP（最多 5 MiB）或 SVG（最多 64 KiB）':
    'Choose PNG, JPEG or WebP (up to 5 MiB), or SVG (up to 64 KiB)',
  '图片处理失败，请选择有效且不超过 2000 万像素的图片，或换用 SVG':
    'Could not process the image. Choose a valid image up to 20 megapixels, or use SVG',
  '保存大小：{size} KiB': 'Stored size: {size} KiB',
  'PNG / JPEG / WebP 最多 5 MiB，自动等比缩小至最长 256 px，压缩后保存（目标 16 KiB，最多 64 KiB）。SVG 最多 64 KiB；也支持 HTTPS 地址。图标存入共享数据库，所有服务副本共用。':
    'PNG / JPEG / WebP up to 5 MiB are resized proportionally to a maximum of 256 px and compressed before saving (target 16 KiB, maximum 64 KiB). SVG is limited to 64 KiB; HTTPS URLs are also supported. Icons are stored in the shared database for all service replicas.',
  'SVG 支持静态图形和文字；样式请写成 fill、stroke 等属性，不支持 style、动画或外部资源。':
    'SVG supports static shapes and text. Use attributes such as fill and stroke; style, animation and external resources are not supported.',
  已上传图片: 'Uploaded image',
  '正在读取图标，请稍候': 'Reading the icon. Please wait',
  '图标地址或 SVG': 'Icon URL or SVG',
  上传图标: 'Upload icon',
  移除图标: 'Remove icon',
  图标预览: 'Icon preview',
  '请输入无账号密码的 HTTPS 图片地址，或安全的 SVG':
    'Enter an HTTPS image URL without credentials, or safe SVG markup',
  '请选择 SVG、PNG、JPEG 或 WebP 图片，大小不超过 64 KiB':
    'Choose an SVG, PNG, JPEG or WebP image no larger than 64 KiB',
  '图片读取失败，请重新选择文件':
    'Could not read the image. Choose the file again',
  '支持 HTTPS 地址、粘贴 SVG 或上传 SVG / PNG / JPEG / WebP（最多 64 KiB）。保存后图标配置存入共享数据库，所有服务副本共用。':
    'Use an HTTPS URL, paste SVG, or upload SVG / PNG / JPEG / WebP (up to 64 KiB). Saved icon settings are stored in the shared database and used by all service replicas.',
};
