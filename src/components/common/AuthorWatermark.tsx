/**
 * 全局背景水印 — 电影厂牌 / 高级品牌质感
 *
 * 设计理念：
 * - 头像作为独立的「厂牌印章」散落在背景中，像电影片头的制片公司 logo
 * - 文字与头像在空间上分离，但通过统一的倾斜角度和透明度形成视觉关联
 * - 模拟高端防伪纸的微雕暗纹 + 影院开场前的幕布质感
 * - 两层叠加：印章层（头像）+ 铭文层（文字），各自独立节奏
 */
export function AuthorWatermark() {
  return (
    <div
      className="pointer-events-none fixed inset-0 z-0 overflow-hidden select-none"
      aria-hidden="true"
    >
      {/* ── 第一层：头像印章散落 ── */}
      <svg className="absolute inset-0 w-full h-full" xmlns="http://www.w3.org/2000/svg">
        <defs>
          {/* 圆形裁剪 */}
          <clipPath id="wm-avatar-clip">
            <circle cx="20" cy="20" r="20" />
          </clipPath>

          {/* 头像印章单元 —— 带光晕环 */}
          <pattern
            id="wm-seal-pattern"
            width="520"
            height="420"
            patternUnits="userSpaceOnUse"
            patternTransform="rotate(-12)"
          >
            {/* 主印章 */}
            <g transform="translate(60, 60)" opacity="0.04" className="dark:opacity-[0.055]">
              <circle cx="20" cy="20" r="24" fill="none" stroke="currentColor" strokeWidth="0.6" className="text-foreground" />
              <circle cx="20" cy="20" r="28" fill="none" stroke="currentColor" strokeWidth="0.3" strokeDasharray="2 3" className="text-foreground" />
              <image
                href="/author-avatar.png"
                x="0" y="0" width="40" height="40"
                clipPath="url(#wm-avatar-clip)"
                preserveAspectRatio="xMidYMid slice"
              />
            </g>

            {/* 偏移半格的幽灵印章（更淡） */}
            <g transform="translate(320, 260)" opacity="0.025" className="dark:opacity-[0.038]">
              <circle cx="16" cy="16" r="19" fill="none" stroke="currentColor" strokeWidth="0.5" className="text-foreground" />
              <clipPath id="wm-avatar-clip-sm">
                <circle cx="16" cy="16" r="16" />
              </clipPath>
              <image
                href="/author-avatar.png"
                x="0" y="0" width="32" height="32"
                clipPath="url(#wm-avatar-clip-sm)"
                preserveAspectRatio="xMidYMid slice"
              />
            </g>
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#wm-seal-pattern)" />
      </svg>

      {/* ── 第二层：铭文字体层（与印章交错，独立节奏）── */}
      <svg className="absolute inset-0 w-full h-full" xmlns="http://www.w3.org/2000/svg">
        <defs>
          <pattern
            id="wm-text-pattern"
            width="600"
            height="350"
            patternUnits="userSpaceOnUse"
            patternTransform="rotate(-12)"
          >
            {/* 作者签名 — 电影字幕风 */}
            <text
              x="260"
              y="120"
              textAnchor="middle"
              className="fill-foreground/[0.028] dark:fill-foreground/[0.04]"
              fontSize="22"
              fontWeight="300"
              fontFamily="'Inter', 'SF Pro Display', -apple-system, BlinkMacSystemFont, system-ui, sans-serif"
              letterSpacing="0.35em"
            >
              COLA
            </text>

            {/* Telegram handle — 副标 */}
            <text
              x="260"
              y="142"
              textAnchor="middle"
              className="fill-foreground/[0.02] dark:fill-foreground/[0.03]"
              fontSize="9.5"
              fontWeight="400"
              fontFamily="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
              letterSpacing="0.2em"
            >
              @gocola
            </text>

            {/* 分隔装饰线 — 像电影海报的金线 */}
            <line
              x1="220" y1="152" x2="300" y2="152"
              className="stroke-foreground/[0.018] dark:stroke-foreground/[0.025]"
              strokeWidth="0.5"
            />

            {/* GitHub 仓库 — 底部小字 */}
            <text
              x="260"
              y="168"
              textAnchor="middle"
              className="fill-foreground/[0.016] dark:fill-foreground/[0.024]"
              fontSize="8"
              fontWeight="400"
              fontFamily="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
              letterSpacing="0.12em"
            >
              gokele/ovh
            </text>

            {/* 偏移位置的品牌短语 */}
            <text
              x="480"
              y="280"
              textAnchor="middle"
              className="fill-foreground/[0.015] dark:fill-foreground/[0.022]"
              fontSize="7.5"
              fontWeight="500"
              fontFamily="'Inter', -apple-system, BlinkMacSystemFont, system-ui, sans-serif"
              letterSpacing="0.4em"
            >
              OPEN SOURCE
            </text>
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#wm-text-pattern)" />
      </svg>

      {/* ── 第三层：中央大尺寸厂牌印记（单次，不平铺）── */}
      <div className="absolute inset-0 flex items-center justify-center opacity-[0.012] dark:opacity-[0.02]">
        <div className="relative" style={{ transform: "rotate(-12deg) scale(1.8)" }}>
          {/* 大头像作为中央 logo mark */}
          <img
            src="/author-avatar.png"
            alt=""
            className="w-28 h-28 rounded-full object-cover"
            style={{
              filter: "grayscale(0.6) contrast(0.8)",
            }}
          />
          {/* 外环 */}
          <div
            className="absolute -inset-4 rounded-full border border-foreground/30"
          />
          <div
            className="absolute -inset-6 rounded-full border border-foreground/15"
            style={{ borderStyle: "dashed", borderWidth: "0.5px" }}
          />
        </div>
      </div>
    </div>
  );
}
