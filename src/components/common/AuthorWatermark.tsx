/**
 * 全局背景水印 — premium action TVC still
 *
 * 把“15s 竖屏高端广告分镜”的动势压缩成一个低透明度背景:
 * 轨迹冲击 -> 定帧碎片 -> 机械光环 -> 涡旋收束 -> 作者署名揭示。
 * 主体仍然只使用本项目的作者头像与 COLA / @gocola / GitHub 信息。
 */

const attackStreaks = [
  { top: 16, left: 5, width: 34, rotate: -8, delay: "0s", opacity: 0.03 },
  { top: 24, left: 68, width: 25, rotate: 172, delay: "-1.7s", opacity: 0.024 },
  { top: 35, left: 2, width: 30, rotate: 5, delay: "-3.1s", opacity: 0.026 },
  { top: 43, left: 75, width: 28, rotate: 188, delay: "-2.2s", opacity: 0.022 },
  { top: 58, left: 9, width: 24, rotate: -12, delay: "-4.6s", opacity: 0.02 },
  { top: 68, left: 63, width: 36, rotate: 176, delay: "-0.9s", opacity: 0.028 },
  { top: 79, left: 16, width: 26, rotate: 9, delay: "-5.4s", opacity: 0.018 },
  { top: 12, left: 42, width: 18, rotate: 92, delay: "-2.9s", opacity: 0.018 },
];

const impactShards = [
  { x: 49, y: 32, rotate: 12, scale: 0.72, delay: "-1.1s" },
  { x: 58, y: 37, rotate: 44, scale: 0.52, delay: "-2.7s" },
  { x: 42, y: 43, rotate: -24, scale: 0.58, delay: "-3.4s" },
  { x: 63, y: 51, rotate: 84, scale: 0.42, delay: "-0.8s" },
  { x: 36, y: 54, rotate: -68, scale: 0.5, delay: "-4.2s" },
  { x: 54, y: 64, rotate: 18, scale: 0.46, delay: "-5.8s" },
  { x: 45, y: 69, rotate: 112, scale: 0.36, delay: "-2s" },
  { x: 31, y: 31, rotate: -38, scale: 0.32, delay: "-6.3s" },
  { x: 70, y: 67, rotate: 55, scale: 0.34, delay: "-3.8s" },
];

const apertureTicks = Array.from({ length: 48 }, (_, i) => i * 7.5);
const sceneCodes = ["ATTACK", "CRUSH", "FORM", "VORTEX", "REVEAL", "TEXTURE", "FOOD", "HERO"];
const sealMarks = [
  { x: 10, y: 18, size: 34, rotate: -14, opacity: 0.016 },
  { x: 78, y: 22, size: 28, rotate: 11, opacity: 0.014 },
  { x: 17, y: 73, size: 26, rotate: 9, opacity: 0.012 },
  { x: 76, y: 78, size: 36, rotate: -18, opacity: 0.016 },
  { x: 48, y: 10, size: 22, rotate: 0, opacity: 0.01 },
];

export function AuthorWatermark() {
  return (
    <div
      className="pointer-events-none fixed inset-0 z-0 overflow-hidden select-none text-foreground"
      aria-hidden="true"
    >
      {/* Macro lens vignette */}
      <div
        className="absolute inset-0 opacity-[0.45]"
        style={{
          background:
            "radial-gradient(ellipse 60% 54% at 50% 48%, transparent 0%, transparent 43%, hsl(var(--background) / 0.62) 100%)",
        }}
      />

      {/* High-speed attack streaks */}
      <div className="absolute inset-0">
        {attackStreaks.map((streak, i) => (
          <div
            key={i}
            className="absolute h-px origin-center"
            style={{
              top: `${streak.top}%`,
              left: `${streak.left}%`,
              width: `${streak.width}%`,
              opacity: streak.opacity,
              transform: `rotate(${streak.rotate}deg)`,
              background:
                "linear-gradient(90deg, transparent 0%, rgba(251,191,36,0.88) 42%, rgba(56,189,248,0.42) 64%, transparent 100%)",
              boxShadow: "0 0 18px rgba(251,191,36,0.3)",
              animation: "wm-attack-streak 9s cubic-bezier(0.2, 0.85, 0.2, 1) infinite",
              animationDelay: streak.delay,
              ["--wm-rotate" as string]: `${streak.rotate}deg`,
            }}
          />
        ))}
      </div>

      {/* Bullet-time impact shards */}
      <div className="absolute inset-0">
        {impactShards.map((shard, i) => (
          <div
            key={i}
            className="absolute"
            style={{
              left: `${shard.x}%`,
              top: `${shard.y}%`,
              width: `${18 * shard.scale}px`,
              height: `${6 * shard.scale}px`,
              opacity: 0.026,
              transform: `translate(-50%, -50%) rotate(${shard.rotate}deg)`,
              background:
                i % 3 === 0
                  ? "rgba(251,191,36,0.65)"
                  : i % 3 === 1
                    ? "rgba(56,189,248,0.5)"
                    : "currentColor",
              clipPath: "polygon(0 50%, 78% 0, 100% 50%, 78% 100%)",
              animation: "wm-shard-freeze 7.5s ease-in-out infinite",
              animationDelay: shard.delay,
              ["--wm-shard-rotate" as string]: `${shard.rotate}deg`,
            }}
          />
        ))}
      </div>

      {/* Cinematic aperture + vortex */}
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="relative h-[min(58vw,520px)] w-[min(58vw,520px)] min-h-[280px] min-w-[280px]">
          <div
            className="absolute inset-0 rounded-full border border-foreground/[0.025] dark:border-foreground/[0.045]"
            style={{
              animation: "wm-aperture-spin 70s linear infinite",
              boxShadow:
                "inset 0 0 70px rgba(56,189,248,0.025), 0 0 90px rgba(251,191,36,0.018)",
            }}
          >
            {apertureTicks.map((deg, i) => (
              <span
                key={deg}
                className="absolute left-1/2 top-1/2 block h-px origin-left bg-foreground/[0.045] dark:bg-foreground/[0.065]"
                style={{
                  width: i % 4 === 0 ? "34px" : "18px",
                  transform: `rotate(${deg}deg) translateX(46%)`,
                }}
              />
            ))}
          </div>

          <div
            className="absolute inset-[10%] rounded-full"
            style={{
              background:
                "conic-gradient(from 18deg, transparent 0deg, rgba(251,191,36,0.045) 24deg, transparent 52deg, transparent 95deg, rgba(56,189,248,0.04) 126deg, transparent 168deg, transparent 235deg, rgba(251,191,36,0.035) 268deg, transparent 315deg, transparent 360deg)",
              maskImage: "radial-gradient(circle, transparent 0 34%, black 43%, transparent 72%)",
              WebkitMaskImage: "radial-gradient(circle, transparent 0 34%, black 43%, transparent 72%)",
              animation: "wm-vortex-turn 26s cubic-bezier(0.45, 0, 0.2, 1) infinite",
            }}
          />

          <div
            className="absolute inset-[22%] rounded-full border border-dashed border-foreground/[0.03] dark:border-foreground/[0.05]"
            style={{ animation: "wm-vortex-turn 42s linear infinite reverse" }}
          />
        </div>
      </div>

      {/* Hero product shot: the actual project watermark avatar */}
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="relative" style={{ transform: "translateY(-4%)" }}>
          <div
            className="absolute rounded-full"
            style={{
              inset: "-72px",
              background:
                "radial-gradient(circle, rgba(251,191,36,0.08) 0%, rgba(251,146,60,0.035) 35%, rgba(56,189,248,0.018) 58%, transparent 72%)",
              filter: "blur(24px)",
              opacity: 0.42,
              animation: "wm-hero-pulse 8s ease-in-out infinite",
            }}
          />

          <div
            className="absolute rounded-full border border-amber-300/[0.055]"
            style={{
              inset: "-28px",
              boxShadow: "0 0 32px rgba(251,191,36,0.045)",
              animation: "wm-hero-lock 11s ease-in-out infinite",
            }}
          />

          <img
            src="/author-avatar.png"
            alt=""
            className="h-24 w-24 rounded-full object-cover opacity-[0.038] dark:opacity-[0.058]"
            style={{
              filter: "saturate(0.78) contrast(0.96) brightness(1.08)",
              boxShadow: "0 0 0 1px rgba(255,255,255,0.07), 0 24px 80px rgba(251,146,60,0.08)",
            }}
          />

          <div
            className="absolute inset-0 rounded-full"
            style={{
              background:
                "linear-gradient(132deg, rgba(255,255,255,0.28) 0%, transparent 34%, transparent 100%)",
              opacity: 0.045,
            }}
          />
        </div>
      </div>

      {/* 8-scene edge slate, borrowed from TVC storyboards */}
      <div className="absolute left-1/2 top-1/2 hidden w-[min(82vw,760px)] -translate-x-1/2 translate-y-[158px] grid-cols-8 gap-2 sm:grid">
        {sceneCodes.map((code, i) => (
          <div
            key={code}
            className="h-8 border-t border-foreground/[0.025] pt-2 text-center font-mono text-[7px] leading-none text-foreground/[0.035] dark:text-foreground/[0.05]"
          >
            <span className="block text-[6px] text-foreground/[0.025] dark:text-foreground/[0.04]">
              0{i + 1}
            </span>
            {code}
          </div>
        ))}
      </div>

      {/* End-credit typography */}
      <div className="absolute inset-0 flex flex-col items-center justify-center" style={{ transform: "translateY(16%)" }}>
        <div
          className="text-[28px] font-[200] leading-none tracking-[0.45em] text-foreground/[0.045] dark:text-foreground/[0.065]"
          style={{ fontFamily: "'Inter', 'SF Pro Display', -apple-system, BlinkMacSystemFont, system-ui, sans-serif" }}
        >
          COLA
        </div>

        <div className="mt-3 mb-2.5 flex items-center gap-3">
          <div className="h-px w-10 bg-amber-300/[0.035]" />
          <div className="h-px w-5 bg-sky-300/[0.035]" />
          <div className="h-px w-10 bg-amber-300/[0.035]" />
        </div>

        <div className="font-mono text-[10px] leading-none tracking-[0.25em] text-foreground/[0.032] dark:text-foreground/[0.045]">
          @gocola
        </div>

        <div className="mt-4 font-mono text-[8.5px] leading-none tracking-[0.15em] text-foreground/[0.022] dark:text-foreground/[0.032]">
          github.com/gokele/ovh
        </div>

        <div
          className="mt-5 text-[7px] font-[500] leading-none tracking-[0.5em] text-foreground/[0.02] dark:text-foreground/[0.03]"
          style={{ fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, system-ui, sans-serif" }}
        >
          OPEN SOURCE CREDITS
        </div>
      </div>

      {/* Distant repeated author seals */}
      <svg className="absolute inset-0 h-full w-full" xmlns="http://www.w3.org/2000/svg">
        {sealMarks.map((seal, i) => (
          <g
            key={i}
            transform={`translate(${seal.x}% ${seal.y}%) rotate(${seal.rotate})`}
            opacity={seal.opacity}
          >
            <circle
              cx="0"
              cy="0"
              r={seal.size / 2}
              fill="none"
              stroke="currentColor"
              strokeWidth="0.45"
              vectorEffect="non-scaling-stroke"
            />
            <path
              d={`M ${-seal.size * 0.22} 0 L ${seal.size * 0.22} 0 M 0 ${-seal.size * 0.22} L 0 ${seal.size * 0.22}`}
              stroke="currentColor"
              strokeWidth="0.35"
              vectorEffect="non-scaling-stroke"
            />
            <text
              x="0"
              y={seal.size * 0.72}
              textAnchor="middle"
              fill="currentColor"
              fontSize="6"
              fontWeight="300"
              letterSpacing="0.28em"
              fontFamily="'Inter', sans-serif"
            >
              COLA
            </text>
          </g>
        ))}
      </svg>

      <style>{`
        @keyframes wm-attack-streak {
          0%, 100% { transform: translate3d(-10px, 0, 0) rotate(var(--wm-rotate, 0deg)) scaleX(0.78); }
          42% { transform: translate3d(16px, 0, 0) rotate(var(--wm-rotate, 0deg)) scaleX(1.08); }
          48% { transform: translate3d(20px, 0, 0) rotate(var(--wm-rotate, 0deg)) scaleX(0.42); }
          56% { transform: translate3d(4px, 0, 0) rotate(var(--wm-rotate, 0deg)) scaleX(0.9); }
        }
        @keyframes wm-shard-freeze {
          0%, 100% { opacity: 0.018; transform: translate(-50%, -50%) rotate(var(--wm-shard-rotate, 0deg)) scale(0.86); }
          46% { opacity: 0.04; transform: translate(-50%, -50%) rotate(var(--wm-shard-rotate, 0deg)) scale(1.14); }
          54% { opacity: 0.026; transform: translate(-50%, -50%) rotate(var(--wm-shard-rotate, 0deg)) scale(1); }
        }
        @keyframes wm-aperture-spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
        @keyframes wm-vortex-turn {
          0%, 100% { transform: rotate(0deg) scale(1); }
          50% { transform: rotate(34deg) scale(1.05); }
        }
        @keyframes wm-hero-pulse {
          0%, 100% { transform: scale(1); opacity: 0.34; }
          50% { transform: scale(1.08); opacity: 0.5; }
        }
        @keyframes wm-hero-lock {
          0%, 100% { transform: scale(1) rotate(0deg); }
          46% { transform: scale(1.07) rotate(3deg); }
          52% { transform: scale(0.98) rotate(-1deg); }
        }
      `}</style>
    </div>
  );
}
