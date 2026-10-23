using Avalonia;
using Avalonia.Headless;
using AfviTools.Tests;

[assembly: AvaloniaTestApplication(typeof(TestAppBuilder))]

namespace AfviTools.Tests;

public static class TestAppBuilder
{
    // Skia with real drawing, so captured frames show the actual Windows 2000 bevels.
    public static AppBuilder BuildAvaloniaApp() =>
        AppBuilder.Configure<App>().UseSkia().UseHeadless(new AvaloniaHeadlessPlatformOptions { UseHeadlessDrawing = false });
}
