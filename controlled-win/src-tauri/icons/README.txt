// 在此目录放入图标文件：
//   32x32.png
//   128x128.png
//   128x128@2x.png
//   icon.icns  (macOS bundle，可选)
//   icon.ico   (Windows 资源)
//   icon.png   (托盘使用)
//
// 简单生成方式：安装 ImageMagick 后
//   magick convert -size 1024x1024 xc:#2f78fa -fill white \
//     -gravity center -pointsize 720 -annotate 0 "L" icon.png
//   magick icon.png -define icon:auto-resize=32,64,128,256 icon.ico
//   magick icon.png -resize 32x32 32x32.png
//   magick icon.png -resize 128x128 128x128.png
//   magick icon.png -resize 256x256 "128x128@2x.png"
//
// 或使用 npx tauri icon icon.png 自动生成全部尺寸。
