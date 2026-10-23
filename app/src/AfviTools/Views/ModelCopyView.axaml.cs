using Avalonia.Controls;
using AfviTools.Services;

namespace AfviTools.Views;

public partial class ModelCopyView : UserControl
{
    public ModelCopyView()
    {
        InitializeComponent();
        ScaleColumns();
        UiSettings.FontSizeChanged += ScaleColumns;
    }

    // DataGrid column widths are plain numbers: scale them with the font size (designed at 14)
    private void ScaleColumns()
    {
        var f = UiSettings.CurrentSize / UiSettings.Default;
        RootGrid.ColumnDefinitions[1].Width = new GridLength(Math.Round(340 * f));
        foreach (var c in ModelGrid.Columns)
            if (c.Tag is int w) c.Width = new DataGridLength(Math.Round(w * f), DataGridLengthUnitType.Pixel);
    }
}
