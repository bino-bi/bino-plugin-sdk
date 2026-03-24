package pluginsdk

import pluginv1 "github.com/bino-bi/bino-plugin-sdk/proto/v1"

func documentsFromProto(pbs []*pluginv1.LintDocument) []Document {
	docs := make([]Document, len(pbs))
	for i, pb := range pbs {
		docs[i] = Document{
			File:     pb.GetFile(),
			Position: int(pb.GetPosition()),
			Kind:     pb.GetKind(),
			Name:     pb.GetName(),
			Raw:      pb.GetRaw(),
		}
	}
	return docs
}

func findingToProto(f Finding, defaultRuleID string) *pluginv1.LintFinding {
	ruleID := f.RuleID
	if ruleID == "" {
		ruleID = defaultRuleID
	}
	return &pluginv1.LintFinding{
		RuleId:   ruleID,
		Message:  f.Message,
		File:     f.File,
		DocIdx:   int32(f.DocIdx),
		Path:     f.Path,
		Line:     int32(f.Line),
		Column:   int32(f.Column),
		Severity: severityToProto(f.Severity),
	}
}

func assetToProto(a Asset) *pluginv1.AssetFile {
	return &pluginv1.AssetFile{
		UrlPath:   a.URLPath,
		Content:   a.Content,
		FilePath:  a.FilePath,
		MediaType: a.MediaType,
		IsModule:  a.IsModule,
	}
}

func commandToProto(cmd Command) *pluginv1.CommandDescriptor {
	pb := &pluginv1.CommandDescriptor{
		Name:  cmd.Name,
		Short: cmd.Short,
		Long:  cmd.Long,
		Usage: cmd.Usage,
	}
	for _, f := range cmd.Flags {
		pb.Flags = append(pb.Flags, &pluginv1.FlagDescriptor{
			Name:         f.Name,
			Shorthand:    f.Shorthand,
			Description:  f.Description,
			DefaultValue: f.DefaultValue,
			Type:         f.Type,
			Required:     f.Required,
		})
	}
	return pb
}

func hookPayloadFromProto(pb *pluginv1.HookPayload) *HookPayload {
	if pb == nil {
		return &HookPayload{}
	}
	hp := &HookPayload{
		HTML:     pb.GetHtml(),
		PDFPath:  pb.GetPdfPath(),
		Metadata: pb.GetMetadata(),
	}
	for _, d := range pb.GetDocuments() {
		hp.Documents = append(hp.Documents, Document{
			File:     d.GetFile(),
			Position: int(d.GetPosition()),
			Kind:     d.GetKind(),
			Name:     d.GetName(),
			Raw:      d.GetRaw(),
		})
	}
	for _, ds := range pb.GetDatasets() {
		hp.Datasets = append(hp.Datasets, Dataset{
			Name:     ds.GetName(),
			JSONRows: ds.GetJsonRows(),
			Columns:  ds.GetColumns(),
		})
	}
	return hp
}

func hookPayloadToProto(hp *HookPayload) *pluginv1.HookPayload {
	if hp == nil {
		return nil
	}
	pb := &pluginv1.HookPayload{
		Html:     hp.HTML,
		PdfPath:  hp.PDFPath,
		Metadata: hp.Metadata,
	}
	for _, d := range hp.Documents {
		pb.Documents = append(pb.Documents, &pluginv1.DocumentPayload{
			File:     d.File,
			Position: int32(d.Position),
			Kind:     d.Kind,
			Name:     d.Name,
			Raw:      d.Raw,
		})
	}
	for _, ds := range hp.Datasets {
		pb.Datasets = append(pb.Datasets, &pluginv1.DatasetPayload{
			Name:     ds.Name,
			JsonRows: ds.JSONRows,
			Columns:  ds.Columns,
		})
	}
	return pb
}

func diagnosticsToProto(diags []Diagnostic) []*pluginv1.Diagnostic {
	if len(diags) == 0 {
		return nil
	}
	pbs := make([]*pluginv1.Diagnostic, len(diags))
	for i, d := range diags {
		pbs[i] = &pluginv1.Diagnostic{
			Source:   d.Source,
			Stage:    d.Stage,
			Message:  d.Message,
			Severity: severityToProto(d.Severity),
		}
	}
	return pbs
}

func severityToProto(s Severity) pluginv1.Severity {
	switch s {
	case Warning:
		return pluginv1.Severity_WARNING
	case Error:
		return pluginv1.Severity_ERROR
	case Info:
		return pluginv1.Severity_INFO
	default:
		return pluginv1.Severity_WARNING
	}
}

func severityFromProto(s pluginv1.Severity) Severity {
	switch s {
	case pluginv1.Severity_WARNING:
		return Warning
	case pluginv1.Severity_ERROR:
		return Error
	case pluginv1.Severity_INFO:
		return Info
	default:
		return Warning
	}
}

func diagnosticsFromProto(pbs []*pluginv1.Diagnostic) []Diagnostic {
	if len(pbs) == 0 {
		return nil
	}
	diags := make([]Diagnostic, len(pbs))
	for i, pb := range pbs {
		diags[i] = Diagnostic{
			Source:   pb.GetSource(),
			Stage:    pb.GetStage(),
			Message:  pb.GetMessage(),
			Severity: severityFromProto(pb.GetSeverity()),
		}
	}
	return diags
}
