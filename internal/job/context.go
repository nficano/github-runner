package job

// This file contains the GitHubContext and RunnerContext types used for
// expression evaluation. The primary Context type and helpers are in job.go.
// This file exists to document the context hierarchy for GitHub Actions:
//
//   github.*  - Repository, event, and run metadata
//   env.*     - Environment variables
//   secrets.* - Secret values (masked in logs)
//   steps.*   - Outputs and conclusions from previous steps
//   runner.*  - Runner machine metadata
//   job.*     - Job-level metadata (scaffold)
//   matrix.*  - Matrix values (scaffold)
//   needs.*   - Dependent job outputs (scaffold)
//
// The Context struct in job.go implements the first five. The remaining
// three (job, matrix, needs) would be added in a full implementation.
