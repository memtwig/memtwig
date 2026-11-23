import * as React from "react";
import {
  MotionConfig,
  motion,
  useReducedMotion,
  useScroll,
  useSpring,
  useTransform,
} from "motion/react";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const highlights = ["Apache 2.0", "Single 14 MB Go binary", "Works 100% offline"];

export function ZeroShowcase() {
  const [mounted, setMounted] = React.useState(false);
  const sectionRef = React.useRef<HTMLDivElement>(null);
  const prefersReduced = useReducedMotion();

  React.useEffect(() => {
    setMounted(true);
  }, []);

  const reduceMotion = mounted && Boolean(prefersReduced);

  const { scrollYProgress } = useScroll({
    target: sectionRef,
    offset: ["start end", "end start"],
  });

  const smoothProgress = useSpring(scrollYProgress, {
    stiffness: 150,
    damping: 26,
  });

  const scale = useTransform(smoothProgress, [0, 0.5, 1], [0.9, 1.04, 0.98]);
  const y = useTransform(smoothProgress, [0, 1], [22, -12]);

  return (
    <MotionConfig reducedMotion="user">
      <div ref={sectionRef} className="relative z-10">
        <motion.p
          initial={{ opacity: 0, y: 12 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, amount: "some" }}
          transition={{ duration: 0.35 }}
          className="text-base text-zinc-400 sm:text-lg"
        >
          Free and open source
        </motion.p>

        <motion.div
          style={reduceMotion ? undefined : { scale, y }}
          className="relative mx-auto mt-2 w-64 overflow-hidden sm:w-96"
        >
          <svg
            viewBox="0 0 420 200"
            className="block h-auto w-full select-none"
            role="img"
            aria-label="Zero dollars"
          >
            <defs>
              <linearGradient id="sculpt-chrome" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#2a2b30" />
                <stop offset="28%" stopColor="#9ca3af" />
                <stop offset="46%" stopColor="#e8f3ff" />
                <stop offset="56%" stopColor="#9be7b5" />
                <stop offset="72%" stopColor="#ebb89a" />
                <stop offset="100%" stopColor="#1c1c20" />
              </linearGradient>
              <linearGradient id="sculpt-rim" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="rgba(255,255,255,0.22)" />
                <stop offset="50%" stopColor="rgba(255,255,255,0.45)" />
                <stop offset="100%" stopColor="rgba(255,255,255,0.06)" />
              </linearGradient>
              <linearGradient id="sculpt-glint" x1="0" y1="0" x2="1" y2="0">
                <stop offset="0%" stopColor="rgba(255,255,255,0)" />
                <stop offset="50%" stopColor="rgba(255,255,255,0.55)" />
                <stop offset="100%" stopColor="rgba(255,255,255,0)" />
              </linearGradient>
              <clipPath id="zero-glyph-clip">
                <text
                  x="210"
                  y="156"
                  textAnchor="middle"
                  fontSize="168"
                  fontWeight="600"
                  letterSpacing="-8"
                >
                  $0
                </text>
              </clipPath>
            </defs>
            <text
              x="210"
              y="156"
              textAnchor="middle"
              fontSize="168"
              fontWeight="600"
              letterSpacing="-8"
              fill="url(#sculpt-chrome)"
              stroke="url(#sculpt-rim)"
              strokeWidth="1.5"
            >
              $0
            </text>
            <g clipPath="url(#zero-glyph-clip)">
              <rect
                x="-60"
                y="0"
                width="140"
                height="200"
                fill="url(#sculpt-glint)"
                className="chrome-sweep"
              />
            </g>
          </svg>
        </motion.div>

        <div className="mt-6 flex flex-wrap items-center justify-center gap-x-7 gap-y-2 text-sm text-zinc-300 sm:text-base">
          {highlights.map((label, idx) => (
            <motion.span
              key={label}
              initial={{ opacity: 0, y: 10 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, amount: "some" }}
              transition={{ duration: 0.3, delay: idx * 0.08 }}
              className="inline-flex items-center gap-2"
            >
              <span className="text-[#58cc7d]" aria-hidden="true">✓</span>
              <span>{label}</span>
            </motion.span>
          ))}
        </div>

        <motion.div
          initial={{ opacity: 0, y: 12 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, amount: "some" }}
          transition={{ duration: 0.35, delay: 0.16 }}
          className="mt-8 flex flex-wrap items-center justify-center gap-3"
        >
          <a href="#install" className={cn(buttonVariants({ variant: "default", size: "lg" }))}>
            Install locally
          </a>
          <a href="/cloud" className={cn(buttonVariants({ variant: "outline", size: "lg" }))}>
            Memtwig Cloud →
          </a>
        </motion.div>
      </div>
    </MotionConfig>
  );
}
