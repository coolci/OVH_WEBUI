/**
 * 全局背景激光刻录防伪水印 (Background Watermark)
 * 纯背景呈现，全屏倾斜平铺，可以看得到但绝不遮挡或违和
 */
export function AuthorWatermark() {
  return (
    <div
      className="pointer-events-none fixed inset-0 z-0 overflow-hidden select-none"
      aria-hidden="true"
    >
      <svg
        className="w-full h-full text-foreground opacity-[0.055] dark:opacity-[0.075]"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          <pattern
            id="author-bg-watermark"
            width="420"
            height="230"
            patternUnits="userSpaceOnUse"
            patternTransform="rotate(-20)"
          >
            <g transform="translate(30, 45)">
              {/* 作者吉祥物/头像 */}
              <image
                href="/author-avatar.png"
                x="0"
                y="-18"
                width="24"
                height="24"
                preserveAspectRatio="xMidYMid slice"
                style={{ clipPath: "circle(12px at 12px 12px)" }}
              />

              {/* 作者名与 Telegram ID */}
              <text
                x="32"
                y="0"
                fill="currentColor"
                fontSize="14"
                fontWeight="700"
                fontFamily="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
                letterSpacing="0.04em"
              >
                cola (@gocola)
              </text>

              {/* GitHub 仓库入口 */}
              <text
                x="32"
                y="22"
                fill="currentColor"
                fontSize="11"
                fontWeight="500"
                fontFamily="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
                letterSpacing="0.06em"
              >
                github.com/gokele/ovh
              </text>

              {/* 鸣谢刻印说明 */}
              <text
                x="0"
                y="48"
                fill="currentColor"
                fontSize="9.5"
                fontWeight="600"
                fontFamily="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
                letterSpacing="0.18em"
              >
                ❖ 鸣谢开源作者 · OPEN SOURCE CREDITS ❖
              </text>

              {/* 防伪刻度线条 */}
              <line
                x1="0"
                y1="60"
                x2="280"
                y2="60"
                stroke="currentColor"
                strokeWidth="0.75"
                strokeDasharray="4 4"
                opacity="0.5"
              />
            </g>
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#author-bg-watermark)" />
      </svg>
    </div>
  );
}
