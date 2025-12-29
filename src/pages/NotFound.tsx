import { AppLayout } from "@/components/layout/AppLayout";
import { Button } from "@/components/ui/button";
import { useLocation, Link } from "react-router-dom";
import { Terminal, ArrowLeft, Home } from "lucide-react";
import { useEffect } from "react";

const NotFound = () => {
  const location = useLocation();

  useEffect(() => {
    console.error("404 Error: User attempted to access non-existent route:", location.pathname);
  }, [location.pathname]);

  return (
    <AppLayout>
      <div className="flex flex-col items-center justify-center min-h-[60vh] text-center">
        {/* ASCII art style 404 */}
        <div className="font-mono text-primary mb-8 text-xs sm:text-sm leading-tight">
          <pre className="opacity-80">
{`
 ██╗  ██╗ ██████╗ ██╗  ██╗
 ██║  ██║██╔═══██╗██║  ██║
 ███████║██║   ██║███████║
 ╚════██║██║   ██║╚════██║
      ██║╚██████╔╝     ██║
      ╚═╝ ╚═════╝      ╚═╝
`}
          </pre>
        </div>

        <div className="terminal-card p-8 max-w-md w-full">
          <div className="flex items-center gap-2 mb-4 text-primary">
            <Terminal className="h-5 w-5" />
            <span className="font-mono text-sm">error_log</span>
          </div>
          
          <div className="font-mono text-sm space-y-2 text-left mb-6">
            <p className="text-muted-foreground">
              <span className="text-primary">$</span> navigate "{location.pathname}"
            </p>
            <p className="text-destructive">
              <span className="text-destructive/70">error:</span> 页面不存在或已被移除
            </p>
            <p className="text-muted-foreground">
              <span className="text-warning/70">hint:</span> 请检查路径是否正确
            </p>
          </div>

          <div className="flex gap-3 justify-center">
            <Button variant="outline" size="sm" onClick={() => window.history.back()}>
              <ArrowLeft className="mr-2 h-4 w-4" />
              返回
            </Button>
            <Link to="/">
              <Button size="sm">
                <Home className="mr-2 h-4 w-4" />
                回到首页
              </Button>
            </Link>
          </div>
        </div>

        <p className="text-xs text-muted-foreground mt-6 font-mono">
          <span className="text-primary">exit_code:</span> 404 | <span className="text-primary">path:</span> {location.pathname}
        </p>
      </div>
    </AppLayout>
  );
};

export default NotFound;
