// Package forger scrubs AI provenance and forges Office metadata.
// Phase 1 target. Bootstrap: contract only, no mutation yet.
package forger

// Profiles: m365 | corporate | clean
// Mutates: docProps/core.xml (dc:creator, lastModifiedBy, created/modified),
// docProps/app.xml (Application, Company, TotalTime, Pages/Words),
// word/settings.xml (rsid pool), word/document.xml (w14:paraId),
// xl/workbook.xml (calcChain delete, dimension fix).
