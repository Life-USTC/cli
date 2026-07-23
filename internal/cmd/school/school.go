package school

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/config"
	"github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
	ustcschool "github.com/Life-USTC/CLI/internal/school"
)

type authFlags struct {
	username      string
	password      string
	totp          string
	undergraduate bool
	graduate      bool
}

var schoolTimeLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

var schoolDebug bool

type schoolCurriculumResult struct {
	Program  ustcschool.Program          `json:"program"`
	Semester ustcschool.Semester         `json:"semester"`
	Items    []ustcschool.CurriculumItem `json:"items"`
}

type schoolSyncSource struct {
	Program ustcschool.Program
	Client  *ustcschool.Client
}

func newMatchSectionCodesBody(codes []string, semesterID string) openapi.MatchSectionCodesJSONRequestBody {
	semesterIDUnion := openapi.MatchSectionCodesRequestSchema_SemesterId{}
	_ = semesterIDUnion.FromMatchSectionCodesRequestSchemaSemesterId0(semesterID)
	return openapi.MatchSectionCodesJSONRequestBody{
		Codes:      codes,
		SemesterId: &semesterIDUnion,
	}
}

func newSemesterScopedSubscriptionSetBody(sectionIDs []int, semesterID string) openapi.BatchUpdateCalendarSubscriptionJSONRequestBody {
	semesterIDUnion := openapi.CalendarSubscriptionBatchRequestSchema_SemesterId{}
	_ = semesterIDUnion.FromCalendarSubscriptionBatchRequestSchemaSemesterId0(semesterID)
	return openapi.BatchUpdateCalendarSubscriptionJSONRequestBody{
		Action:     openapi.CalendarSubscriptionBatchRequestSchemaActionSet,
		SectionIds: &sectionIDs,
		SemesterId: &semesterIDUnion,
	}
}

func NewCmdSchool() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "school",
		Short: "Read data from official USTC sites",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().BoolVar(&schoolDebug, "debug", false, "Print school fetch and sync timing details to stderr")

	cmd.AddCommand(
		newCmdSchoolSemesters(),
		newCmdSchoolCurriculum(),
		newCmdSchoolExam(),
		newCmdSchoolScore(),
		newCmdSchoolHomework(),
		newCmdSchoolSync(),
	)

	return cmd
}

func newCmdSchoolSemesters() *cobra.Command {
	var auth authFlags

	cmd := &cobra.Command{
		Use:   "semesters",
		Short: "List semesters from official USTC systems",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer debugStep("school semesters")()
			client, err := newSchoolClient(auth)
			if err != nil {
				return err
			}
			semesters, err := client.FetchSemesters(cmd.Context())
			if err != nil {
				return err
			}

			rows := make([]map[string]any, 0, len(semesters))
			for _, semester := range semesters {
				rows = append(rows, map[string]any{
					"id":        semester.ID,
					"code":      semester.Code,
					"name":      semester.Name(),
					"startDate": semester.StartDate,
					"endDate":   semester.EndDate,
					"isCurrent": semester.IsLast,
				})
			}

			return output.OutputList(
				semesters,
				rows,
				[]output.Column{
					{Key: "id", Header: "ID"},
					{Key: "code", Header: "Code"},
					{Key: "name", Header: "Semester"},
					{Key: "startDate", Header: "Start"},
					{Key: "endDate", Header: "End"},
					{Key: "isCurrent", Header: "Current"},
				},
				0,
				0,
			)
		},
	}

	addAuthFlags(cmd, &auth)
	return cmd
}

func newCmdSchoolCurriculum() *cobra.Command {
	var auth authFlags
	var semesterID int

	cmd := &cobra.Command{
		Use:   "curriculum",
		Short: "Read your current USTC curriculum from jw.ustc.edu.cn",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer debugStep("school curriculum")()
			client, err := newSchoolClient(auth)
			if err != nil {
				return err
			}
			semester, items, err := client.FetchCurriculum(cmd.Context(), semesterID)
			if err != nil {
				return err
			}

			rows := make([]map[string]any, 0, len(items))
			for _, item := range items {
				rows = append(rows, map[string]any{
					"lessonCode": item.LessonCode,
					"courseCode": item.CourseCode,
					"course":     item.CourseName,
					"credits":    item.Credits,
					"teachers":   item.TeacherList(),
					"schedule":   item.Schedule,
				})
			}

			if !output.IsJSON() {
				output.KVWithTitle([]output.KVPair{
					{Key: "ID", Value: semester.ID},
					{Key: "Name", Value: semester.Name()},
					{Key: "Code", Value: semester.Code, SkipEmpty: true},
				}, "Semester")
			}

			return output.OutputList(
				map[string]any{"semester": semester, "items": items},
				rows,
				[]output.Column{
					{Key: "lessonCode", Header: "Lesson Code"},
					{Key: "courseCode", Header: "Course Code"},
					{Key: "course", Header: "Course"},
					{Key: "credits", Header: "Credits"},
					{Key: "teachers", Header: "Teachers"},
					{Key: "schedule", Header: "Schedule"},
				},
				0,
				0,
			)
		},
	}

	addAuthFlags(cmd, &auth)
	cmd.Flags().IntVar(&semesterID, "semester-id", 0, "Catalog semester ID (defaults to current semester)")
	return cmd
}

func newCmdSchoolExam() *cobra.Command {
	var auth authFlags
	var semesterID int

	cmd := &cobra.Command{
		Use:   "exam",
		Short: "Read your exam arrangements from jw.ustc.edu.cn",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer debugStep("school exam")()
			client, err := newSchoolClient(auth)
			if err != nil {
				return err
			}
			items, err := client.FetchExams(cmd.Context(), semesterID)
			if err != nil {
				return err
			}

			rows := make([]map[string]any, 0, len(items))
			for _, item := range items {
				rows = append(rows, map[string]any{
					"course":     item.CourseName,
					"lessonCode": item.LessonCode,
					"type":       item.ExamType,
					"datetime":   item.DateTime,
					"location":   item.Location,
					"seat":       item.Seat,
					"status":     item.Status,
				})
			}

			return output.OutputList(
				items,
				rows,
				[]output.Column{
					{Key: "course", Header: "Course"},
					{Key: "lessonCode", Header: "Lesson Code"},
					{Key: "type", Header: "Type"},
					{Key: "datetime", Header: "Date / Time"},
					{Key: "location", Header: "Location"},
					{Key: "seat", Header: "Seat"},
					{Key: "status", Header: "Status"},
				},
				0,
				0,
			)
		},
	}

	addAuthFlags(cmd, &auth)
	cmd.Flags().IntVar(&semesterID, "semester-id", 0, "Graduate semester ID (defaults to current semester)")
	return cmd
}

func newCmdSchoolScore() *cobra.Command {
	var auth authFlags

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Read your scores from jw.ustc.edu.cn",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer debugStep("school score")()
			client, err := newSchoolClient(auth)
			if err != nil {
				return err
			}
			report, err := client.FetchScores(cmd.Context())
			if err != nil {
				return err
			}

			rows := make([]map[string]any, 0, len(report.Items))
			for _, item := range report.Items {
				rows = append(rows, map[string]any{
					"semester":   firstNonEmpty(item.SemesterName, strconv.Itoa(item.SemesterID)),
					"lessonCode": item.LessonCode,
					"courseCode": item.CourseCode,
					"course":     item.CourseName,
					"credits":    item.Credits,
					"gp":         item.GradePoint,
					"score":      item.Score,
					"gradeText":  item.GradeText,
				})
			}

			if !output.IsJSON() && len(report.Summary) > 0 {
				var summary map[string]any
				if err := json.Unmarshal(report.Summary, &summary); err == nil {
					pairs := make([]output.KVPair, 0, len(summary))
					for key, value := range summary {
						pairs = append(pairs, output.KVPair{Key: key, Value: value, SkipEmpty: true})
					}
					output.KVWithTitle(pairs, "Summary")
				}
			}

			return output.OutputList(
				report,
				rows,
				[]output.Column{
					{Key: "semester", Header: "Semester"},
					{Key: "lessonCode", Header: "Lesson Code"},
					{Key: "courseCode", Header: "Course Code"},
					{Key: "course", Header: "Course"},
					{Key: "credits", Header: "Credits"},
					{Key: "gp", Header: "GP"},
					{Key: "score", Header: "Score"},
					{Key: "gradeText", Header: "Detail"},
				},
				0,
				0,
			)
		},
	}

	addAuthFlags(cmd, &auth)
	return cmd
}

func newCmdSchoolHomework() *cobra.Command {
	var auth authFlags

	cmd := &cobra.Command{
		Use:   "homework [command]",
		Short: "Read Blackboard homework and calendar items",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer debugStep("school homework")()
			client, err := newSchoolClient(auth)
			if err != nil {
				return err
			}
			items, err := client.FetchHomework(cmd.Context())
			if err != nil {
				return err
			}

			rows := make([]map[string]any, 0, len(items))
			for _, item := range items {
				rows = append(rows, map[string]any{
					"course":     item.CourseName,
					"lessonCode": item.LessonCode,
					"title":      item.Title,
					"startAt":    item.StartAt,
					"endAt":      item.EndAt,
					"status":     item.Status,
				})
			}

			return output.OutputList(
				items,
				rows,
				[]output.Column{
					{Key: "course", Header: "Course"},
					{Key: "lessonCode", Header: "Lesson Code"},
					{Key: "title", Header: "Title"},
					{Key: "startAt", Header: "Start"},
					{Key: "endAt", Header: "End"},
					{Key: "status", Header: "Status"},
				},
				0,
				0,
			)
		},
	}

	addAuthFlags(cmd, &auth)
	cmd.AddCommand(newCmdSchoolHomeworkSync())
	return cmd
}

func newCmdSchoolHomeworkSync() *cobra.Command {
	var auth authFlags
	var allPrograms bool
	var semesterID int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync school homework to Life@USTC",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer debugStep("school homework sync")()
			if allPrograms && semesterID != 0 {
				return fmt.Errorf("--semester-id cannot be used with --all-programs")
			}
			var sources []schoolSyncSource
			if err := withDebugStep("resolve school sync sources", func() error {
				var err error
				sources, err = newSchoolSyncSources(auth, allPrograms)
				return err
			}); err != nil {
				return err
			}
			var apiClient *api.TypedClient
			if err := withDebugStep("create Life@USTC API client", func() error {
				var err error
				apiClient, err = api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
				return err
			}); err != nil {
				return err
			}
			var lifeSemesterRaw any
			if err := withDebugStep("Life@USTC list semesters", func() error {
				var err error
				lifeSemesterRaw, err = api.ParseResponseRaw(apiClient.ListSemesters(cmd.Context(), &openapi.ListSemestersParams{
					Limit: int64Ptr(200),
				}))
				return err
			}); err != nil {
				return err
			}

			var results []homeworkSyncResult
			for _, source := range sources {
				result, err := syncHomeworkForSource(cmd, apiClient, source, lifeSemesterRaw, semesterID, dryRun)
				if err != nil {
					return err
				}
				results = append(results, result)
			}
			result := mergeHomeworkSyncResults(results, dryRun)

			if output.IsJSON() {
				return output.JSON(result)
			}

			output.KVWithTitle([]output.KVPair{
				{Key: "Programs", Value: strings.Join(homeworkSyncPrograms(results), ", ")},
				{Key: "School Homework", Value: len(result.SchoolHomework)},
				{Key: "Created", Value: len(result.Created)},
				{Key: "Matched", Value: len(result.Matched)},
				{Key: "Unmatched", Value: len(result.Unmatched)},
				{Key: "Dry Run", Value: dryRun},
			}, "Homework Sync")

			rows := homeworkSyncRows(result)
			return output.OutputList(result, rows, []output.Column{
				{Key: "action", Header: "Action"},
				{Key: "course", Header: "Course"},
				{Key: "title", Header: "Title"},
				{Key: "due", Header: "Due"},
				{Key: "section", Header: "Section"},
				{Key: "completion", Header: "Done"},
			}, len(rows), 0)
		},
	}

	addAuthFlags(cmd, &auth)
	cmd.Flags().BoolVar(&allPrograms, "all-programs", false, "Sync undergraduate and graduate homework in one run")
	cmd.Flags().IntVar(&semesterID, "semester-id", 0, "Catalog semester ID (defaults to current semester)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without creating homework or updating completion")
	return cmd
}

func newCmdSchoolSync() *cobra.Command {
	var auth authFlags
	var allPrograms bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "One-way sync all school lessons to Life@USTC",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer debugStep("school section sync")()
			sources, err := newSchoolSyncSources(auth, allPrograms)
			if err != nil {
				return err
			}
			var curricula []schoolCurriculumResult
			var skipped []map[string]any
			for _, source := range sources {
				sourceCurricula, sourceSkipped, err := fetchAllCurricula(cmd, source)
				if err != nil {
					return err
				}
				curricula = append(curricula, sourceCurricula...)
				skipped = append(skipped, sourceSkipped...)
			}
			if len(curricula) == 0 {
				return fmt.Errorf("no school semesters with lesson codes were found")
			}

			apiClient, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}

			lifeSemesterRaw, err := api.ParseResponseRaw(apiClient.ListSemesters(cmd.Context(), &openapi.ListSemestersParams{
				Limit: int64Ptr(200),
			}))
			if err != nil {
				return err
			}

			allSectionIDs := map[int]struct{}{}
			sectionIDsBySemester := map[string]map[int]struct{}{}
			var allCurriculum []ustcschool.CurriculumItem
			var allCodes []string
			var matchedCodes []any
			var unmatchedCodes []any
			var sections []any
			var semesterResults []map[string]any
			for _, curriculum := range curricula {
				codes := uniqueLessonCodes(curriculum.Items)
				if len(codes) == 0 {
					continue
				}
				lifeSemesterID, lifeSemester, ok := resolveLifeSemester(lifeSemesterRaw, curriculum.Semester)
				if !ok {
					skipped = append(skipped, map[string]any{
						"program":  curriculum.Program,
						"semester": curriculum.Semester,
						"reason":   "could not map to a Life@USTC semester",
					})
					continue
				}

				matchRaw, err := api.ParseResponseRaw(apiClient.MatchSectionCodes(cmd.Context(), newMatchSectionCodesBody(codes, lifeSemesterID)))
				if err != nil {
					return err
				}
				matchMap := cmdutil.AsMap(matchRaw)
				semesterSectionIDs := sectionIDsBySemester[lifeSemesterID]
				if semesterSectionIDs == nil {
					semesterSectionIDs = map[int]struct{}{}
					sectionIDsBySemester[lifeSemesterID] = semesterSectionIDs
				}
				for _, sectionID := range extractSectionIDs(matchMap["sections"]) {
					allSectionIDs[sectionID] = struct{}{}
					semesterSectionIDs[sectionID] = struct{}{}
				}
				allCurriculum = append(allCurriculum, curriculum.Items...)
				allCodes = append(allCodes, codes...)
				matchedCodes = append(matchedCodes, anySlice(matchMap["matchedCodes"])...)
				unmatchedCodes = append(unmatchedCodes, anySlice(matchMap["unmatchedCodes"])...)
				sections = append(sections, anySlice(matchMap["sections"])...)
				semesterResults = append(semesterResults, map[string]any{
					"program":        curriculum.Program,
					"semester":       curriculum.Semester,
					"lifeSemester":   lifeSemester,
					"codes":          codes,
					"matchedCodes":   matchMap["matchedCodes"],
					"unmatchedCodes": matchMap["unmatchedCodes"],
					"sections":       matchMap["sections"],
				})
			}

			sectionIDs := sortedIntsFromSet(allSectionIDs)
			result := map[string]any{
				"curricula":       curricula,
				"curriculum":      allCurriculum,
				"codes":           allCodes,
				"matchedCodes":    matchedCodes,
				"unmatchedCodes":  unmatchedCodes,
				"sections":        sections,
				"sectionIds":      sectionIDs,
				"semesterResults": semesterResults,
				"skipped":         skipped,
				"dryRun":          dryRun,
			}

			if !dryRun && len(sectionIDs) > 0 {
				semesterIDs := make([]string, 0, len(sectionIDsBySemester))
				for lifeSemesterID := range sectionIDsBySemester {
					semesterIDs = append(semesterIDs, lifeSemesterID)
				}
				slices.Sort(semesterIDs)
				for _, lifeSemesterID := range semesterIDs {
					semesterSectionIDs := sortedIntsFromSet(sectionIDsBySemester[lifeSemesterID])
					if len(semesterSectionIDs) == 0 {
						continue
					}
					subscribeRaw, err := api.ParseResponseRaw(apiClient.BatchUpdateCalendarSubscription(cmd.Context(), newSemesterScopedSubscriptionSetBody(semesterSectionIDs, lifeSemesterID)))
					if err != nil {
						return err
					}
					result["subscription"] = subscribeRaw
				}
			}

			if output.IsJSON() {
				return output.JSON(result)
			}

			output.KVWithTitle([]output.KVPair{
				{Key: "School Semesters", Value: len(curricula)},
				{Key: "Skipped Semesters", Value: len(skipped)},
				{Key: "Lesson Code Count", Value: len(allCodes)},
				{Key: "Matched Sections", Value: len(sectionIDs)},
				{Key: "All Programs", Value: allPrograms},
				{Key: "Dry Run", Value: dryRun},
			}, "Sync")

			if len(unmatchedCodes) > 0 {
				output.KVWithTitle([]output.KVPair{{Key: "Codes", Value: unmatchedCodes, SkipEmpty: true}}, "Unmatched Codes")
			}

			rows := cmdutil.RowsFromAny(sections)
			return output.OutputList(
				sections,
				rows,
				[]output.Column{
					{Key: "id", Header: "ID"},
					{Key: "code", Header: "Code"},
					{Key: "course.nameCn", Header: "Course"},
					{Key: "title", Header: "Title"},
					{Key: "semester.nameCn", Header: "Semester"},
				},
				0,
				0,
			)
		},
	}

	addAuthFlags(cmd, &auth)
	cmd.Flags().BoolVar(&allPrograms, "all-programs", false, "Sync undergraduate and graduate lessons in one subscription update")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Crawl and match without updating Life@USTC calendar subscriptions")
	return cmd
}

func addAuthFlags(cmd *cobra.Command, auth *authFlags) {
	cmd.Flags().StringVar(&auth.username, "username", "", "USTC passport username (env: PASSPORT_UNDERGRADUATE_USERNAME or PASSPORT_GRADUATE_USERNAME)")
	cmd.Flags().StringVar(&auth.password, "password", "", "USTC passport password (env: PASSPORT_PASSWORD)")
	cmd.Flags().StringVar(&auth.totp, "totp", "", "USTC OTP code or TOTP secret/otpauth URL (env: PASSPORT_TOTP)")
	cmd.Flags().BoolVar(&auth.undergraduate, "undergraduate", false, "Use undergraduate-system credentials and endpoints")
	cmd.Flags().BoolVar(&auth.graduate, "graduate", false, "Use graduate-system credentials and endpoints")
}

func debugLog(format string, args ...any) {
	if !schoolDebug {
		return
	}
	fmt.Fprintf(os.Stderr, "[school debug] "+format+"\n", args...)
}

func debugStep(name string) func() {
	if !schoolDebug {
		return func() {}
	}
	start := time.Now()
	debugLog("start %s", name)
	return func() {
		debugLog("done %s (%s)", name, time.Since(start).Round(time.Millisecond))
	}
}

func withDebugStep(name string, fn func() error) error {
	done := debugStep(name)
	defer done()
	return fn()
}

func configureSchoolDebug() {
	if !schoolDebug {
		ustcschool.SetDebugLogger(nil)
		return
	}
	ustcschool.SetDebugLogger(debugLog)
}

func newSchoolClient(auth authFlags) (*ustcschool.Client, error) {
	configureSchoolDebug()
	programs, err := selectSchoolPrograms(auth, false)
	if err != nil {
		return nil, err
	}
	if len(programs) != 1 {
		return nil, fmt.Errorf("this command reads one school program at a time; use --undergraduate or --graduate")
	}
	program := programs[0]

	creds, err := ustcschool.ResolveCredentialsForProgram(program, auth.username, auth.password, auth.totp)
	if err != nil {
		return nil, err
	}
	return ustcschool.NewClient(creds, program), nil
}

func newSchoolSyncSources(auth authFlags, allPrograms bool) ([]schoolSyncSource, error) {
	configureSchoolDebug()
	programs, err := selectSchoolPrograms(auth, allPrograms)
	if err != nil {
		return nil, err
	}

	sources := make([]schoolSyncSource, 0, len(programs))
	for _, program := range programs {
		creds, err := ustcschool.ResolveCredentialsForProgram(program, auth.username, auth.password, auth.totp)
		if err != nil {
			return nil, err
		}
		sources = append(sources, schoolSyncSource{
			Program: program,
			Client:  ustcschool.NewClient(creds, program),
		})
	}
	return sources, nil
}

func selectSchoolPrograms(auth authFlags, allPrograms bool) ([]ustcschool.Program, error) {
	if allPrograms {
		return []ustcschool.Program{ustcschool.ProgramUndergraduate, ustcschool.ProgramGraduate}, nil
	}
	if auth.undergraduate && auth.graduate {
		return []ustcschool.Program{ustcschool.ProgramUndergraduate, ustcschool.ProgramGraduate}, nil
	}
	if auth.undergraduate {
		return []ustcschool.Program{ustcschool.ProgramUndergraduate}, nil
	}
	if auth.graduate {
		return []ustcschool.Program{ustcschool.ProgramGraduate}, nil
	}
	if programs, err := configuredSchoolPrograms(); err != nil {
		return nil, err
	} else if len(programs) > 0 {
		return programs, nil
	}
	if programs := ustcschool.DetectCredentialPrograms(); len(programs) > 0 {
		return programs, nil
	}
	return []ustcschool.Program{ustcschool.ProgramUndergraduate}, nil
}

func configuredSchoolPrograms() ([]ustcschool.Program, error) {
	values := config.GetSchoolPrograms()
	programs := make([]ustcschool.Program, 0, len(values))
	seen := map[ustcschool.Program]struct{}{}
	for _, value := range values {
		var program ustcschool.Program
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "", "none":
			continue
		case "undergrad", "undergraduate":
			program = ustcschool.ProgramUndergraduate
		case "grad", "graduate":
			program = ustcschool.ProgramGraduate
		default:
			return nil, fmt.Errorf("invalid configured school program %q; run `life-ustc config set-school-programs undergraduate,graduate`", value)
		}
		if _, ok := seen[program]; ok {
			continue
		}
		seen[program] = struct{}{}
		programs = append(programs, program)
	}
	return programs, nil
}

func uniqueLessonCodes(items []ustcschool.CurriculumItem) []string {
	set := make(map[string]struct{}, len(items))
	var codes []string
	for _, item := range items {
		code := strings.TrimSpace(item.LessonCode)
		if code == "" {
			continue
		}
		if _, ok := set[code]; ok {
			continue
		}
		set[code] = struct{}{}
		codes = append(codes, code)
	}
	return codes
}

func fetchAllCurricula(cmd *cobra.Command, source schoolSyncSource) ([]schoolCurriculumResult, []map[string]any, error) {
	var semesters []ustcschool.Semester
	if err := withDebugStep(fmt.Sprintf("%s fetch semesters", source.Program), func() error {
		var err error
		semesters, err = source.Client.FetchSemesters(cmd.Context())
		return err
	}); err != nil {
		return nil, nil, err
	}
	debugLog("%s semesters=%d", source.Program, len(semesters))

	var curricula []schoolCurriculumResult
	var skipped []map[string]any
	seen := map[int]struct{}{}
	for _, semester := range semesters {
		if _, ok := seen[semester.ID]; ok {
			continue
		}
		seen[semester.ID] = struct{}{}

		var selected ustcschool.Semester
		var items []ustcschool.CurriculumItem
		stepName := fmt.Sprintf("%s fetch curriculum semester=%d", source.Program, semester.ID)
		if err := withDebugStep(stepName, func() error {
			var err error
			selected, items, err = source.Client.FetchCurriculum(cmd.Context(), semester.ID)
			return err
		}); err != nil {
			if ustcschool.IsNoLessonData(err) {
				skipped = append(skipped, map[string]any{
					"program":  source.Program,
					"semester": semester,
					"reason":   err.Error(),
				})
				continue
			}
			return nil, nil, err
		}
		debugLog("%s semester=%d lessonCodes=%d items=%d", source.Program, selected.ID, len(uniqueLessonCodes(items)), len(items))
		if len(uniqueLessonCodes(items)) == 0 {
			skipped = append(skipped, map[string]any{
				"program":  source.Program,
				"semester": selected,
				"reason":   "no lesson codes",
			})
			continue
		}
		curricula = append(curricula, schoolCurriculumResult{
			Program:  source.Program,
			Semester: selected,
			Items:    items,
		})
	}
	return curricula, skipped, nil
}

func anySlice(value any) []any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return items
}

func uniqueStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := set[value]; ok {
			continue
		}
		set[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func sortedIntsFromSet(set map[int]struct{}) []int {
	ids := make([]int, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

type homeworkSyncResult struct {
	Program           ustcschool.Program        `json:"program,omitempty"`
	Semester          ustcschool.Semester       `json:"semester"`
	LifeSemester      map[string]any            `json:"lifeSemester,omitempty"`
	SchoolHomework    []ustcschool.HomeworkItem `json:"schoolHomework"`
	Created           []homeworkSyncItemResult  `json:"created"`
	Matched           []homeworkSyncItemResult  `json:"matched"`
	CompletionUpdated []homeworkSyncItemResult  `json:"completionUpdated"`
	Skipped           []homeworkSyncItemResult  `json:"skipped"`
	Unmatched         []homeworkSyncItemResult  `json:"unmatched"`
	Results           []homeworkSyncResult      `json:"results,omitempty"`
	DryRun            bool                      `json:"dryRun"`
}

type homeworkSyncItemResult struct {
	SchoolHomework   ustcschool.HomeworkItem `json:"schoolHomework"`
	LifeHomework     map[string]any          `json:"lifeHomework,omitempty"`
	Section          map[string]any          `json:"section,omitempty"`
	Action           string                  `json:"action"`
	Reason           string                  `json:"reason,omitempty"`
	Completion       bool                    `json:"completion"`
	CompletionKnown  bool                    `json:"completionKnown"`
	CompletionAction string                  `json:"completionAction,omitempty"`
}

func newHomeworkSyncResult(program ustcschool.Program, semester ustcschool.Semester, lifeSemester map[string]any, homework []ustcschool.HomeworkItem, dryRun bool) homeworkSyncResult {
	return homeworkSyncResult{
		Program:        program,
		Semester:       semester,
		LifeSemester:   lifeSemester,
		SchoolHomework: homework,
		Created:        []homeworkSyncItemResult{},
		Matched:        []homeworkSyncItemResult{},
		Unmatched:      []homeworkSyncItemResult{},
		Skipped:        []homeworkSyncItemResult{},
		DryRun:         dryRun,
	}
}

func syncHomeworkForSource(cmd *cobra.Command, apiClient *api.TypedClient, source schoolSyncSource, lifeSemesterRaw any, semesterID int, dryRun bool) (homeworkSyncResult, error) {
	defer debugStep(fmt.Sprintf("%s homework sync source", source.Program))()
	var homework []ustcschool.HomeworkItem
	if err := withDebugStep(fmt.Sprintf("%s fetch school homework", source.Program), func() error {
		var err error
		homework, err = source.Client.FetchHomework(cmd.Context())
		return err
	}); err != nil {
		return homeworkSyncResult{}, err
	}
	debugLog("%s homework items=%d", source.Program, len(homework))

	var sectionsByHomeworkCode map[string]map[string]any
	if err := withDebugStep(fmt.Sprintf("%s map sections by homework code", source.Program), func() error {
		var err error
		sectionsByHomeworkCode, err = homeworkSectionsByCode(cmd, apiClient, lifeSemesterRaw, homework)
		return err
	}); err != nil {
		return homeworkSyncResult{}, err
	}
	debugLog("%s homework-code section mappings=%d", source.Program, len(sectionsByHomeworkCode))

	needsCourseMapping := false
	for _, item := range homework {
		if _, ok := sectionsByHomeworkCode[homeworkSectionKey(item)]; !ok {
			needsCourseMapping = true
			break
		}
	}

	var semester ustcschool.Semester
	var lifeSemester map[string]any
	sectionsByCourse := map[string]map[string]any{}
	if needsCourseMapping {
		if err := withDebugStep(fmt.Sprintf("%s map sections by course", source.Program), func() error {
			var err error
			semester, lifeSemester, sectionsByCourse, err = homeworkSectionsByCourse(cmd, apiClient, source, lifeSemesterRaw, semesterID)
			return err
		}); err != nil {
			return homeworkSyncResult{}, err
		}
		debugLog("%s course section mappings=%d", source.Program, len(sectionsByCourse))
	} else {
		debugLog("%s skipped course section mapping; homework codes covered all items", source.Program)
	}

	result := newHomeworkSyncResult(source.Program, semester, lifeSemester, homework, dryRun)
	existingBySection := map[string][]map[string]any{}
	for _, item := range homework {
		section, ok := sectionsByHomeworkCode[homeworkSectionKey(item)]
		if !ok {
			section, ok = sectionsByCourse[normalizeHomeworkKey(item.CourseName)]
		}
		if !ok {
			result.Unmatched = append(result.Unmatched, homeworkSyncItemResult{
				SchoolHomework: item,
				Action:         "unmatched",
				Reason:         "no matched Life@USTC section for course",
			})
			continue
		}

		sectionID, ok := anyIntString(section["id"])
		if !ok || sectionID == "" {
			result.Unmatched = append(result.Unmatched, homeworkSyncItemResult{
				SchoolHomework: item,
				Section:        section,
				Action:         "unmatched",
				Reason:         "matched section has no id",
			})
			continue
		}

		existing, ok := existingBySection[sectionID]
		if !ok {
			if err := withDebugStep(fmt.Sprintf("Life@USTC list homework section=%s", sectionID), func() error {
				var err error
				existing, err = fetchLifeHomeworksForSection(cmd, apiClient, sectionID)
				return err
			}); err != nil {
				return homeworkSyncResult{}, err
			}
			existingBySection[sectionID] = existing
		}

		matched := matchLifeHomework(existing, item)
		homeworkID := anyString(matched["id"])
		action := "matched"
		if homeworkID == "" {
			action = "created"
			if !dryRun {
				created, err := createLifeHomework(cmd, apiClient, sectionID, item)
				if err != nil {
					return homeworkSyncResult{}, err
				}
				matched = created
				homeworkID = anyString(created["id"])
				if homeworkID == "" {
					return homeworkSyncResult{}, fmt.Errorf("created homework for %q did not return an id", item.Title)
				}
				existingBySection[sectionID] = append(existingBySection[sectionID], created)
			}
		}

		completed, completionKnown := schoolHomeworkCompletion(item)
		completionAction := "unknown"
		if completionKnown {
			completionAction = "planned"
		}
		if completionKnown && !dryRun && homeworkID != "" {
			if err := setLifeHomeworkCompletion(cmd, apiClient, homeworkID, completed); err != nil {
				return homeworkSyncResult{}, err
			}
			completionAction = "updated"
		}

		row := homeworkSyncItemResult{
			SchoolHomework:   item,
			LifeHomework:     matched,
			Section:          section,
			Action:           action,
			Completion:       completed,
			CompletionKnown:  completionKnown,
			CompletionAction: completionAction,
		}
		if action == "created" {
			result.Created = append(result.Created, row)
		} else {
			result.Matched = append(result.Matched, row)
		}
		if completionKnown {
			result.CompletionUpdated = append(result.CompletionUpdated, row)
		}
	}
	return result, nil
}

func homeworkSectionsByCode(cmd *cobra.Command, apiClient *api.TypedClient, lifeSemesterRaw any, homework []ustcschool.HomeworkItem) (map[string]map[string]any, error) {
	codesByLifeSemester := map[string][]string{}
	blackboardSemesterByLifeCode := map[string]string{}
	for _, item := range homework {
		code := strings.TrimSpace(item.LessonCode)
		semesterCode := strings.TrimSpace(item.SemesterCode)
		if code == "" || semesterCode == "" {
			continue
		}
		lifeSemesterID, ok := resolveBlackboardLifeSemester(lifeSemesterRaw, semesterCode)
		if !ok {
			continue
		}
		codesByLifeSemester[lifeSemesterID] = append(codesByLifeSemester[lifeSemesterID], code)
		blackboardSemesterByLifeCode[lifeSemesterID+"|"+code] = semesterCode
	}

	out := map[string]map[string]any{}
	for lifeSemesterID, codes := range codesByLifeSemester {
		codes = uniqueStrings(codes)
		matchRaw, err := api.ParseResponseRaw(apiClient.MatchSectionCodes(cmd.Context(), newMatchSectionCodesBody(codes, lifeSemesterID)))
		if err != nil {
			return nil, err
		}
		for code, section := range sectionsByCode(cmdutil.AsMap(matchRaw)["sections"]) {
			semesterCode := blackboardSemesterByLifeCode[lifeSemesterID+"|"+code]
			out[code+"|"+semesterCode] = section
		}
	}
	return out, nil
}

func resolveBlackboardLifeSemester(raw any, blackboardSemesterCode string) (string, bool) {
	year, term, ok := parseBlackboardSemesterCode(blackboardSemesterCode)
	if !ok {
		return "", false
	}
	result := cmdutil.NewListResult(raw, "data")
	for _, row := range result.Rows {
		id, ok := anyIntString(row["id"])
		if !ok {
			continue
		}
		name := anyString(row["nameCn"])
		if strings.Contains(name, year+"年") && strings.Contains(name, term) {
			return id, true
		}
	}
	return "", false
}

func parseBlackboardSemesterCode(code string) (string, string, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return "", "", false
	}
	year := code[:4]
	for _, r := range year {
		if r < '0' || r > '9' {
			return "", "", false
		}
	}
	switch code[4:] {
	case "SP":
		return year, "春季", true
	case "SU":
		return year, "夏季", true
	case "FA":
		return year, "秋季", true
	default:
		return "", "", false
	}
}

func homeworkSectionKey(item ustcschool.HomeworkItem) string {
	code := strings.TrimSpace(item.LessonCode)
	semester := strings.TrimSpace(item.SemesterCode)
	if code == "" || semester == "" {
		return ""
	}
	return code + "|" + semester
}

func homeworkSectionsByCourse(cmd *cobra.Command, apiClient *api.TypedClient, source schoolSyncSource, lifeSemesterRaw any, semesterID int) (ustcschool.Semester, map[string]any, map[string]map[string]any, error) {
	curricula, err := homeworkSyncCurricula(cmd, source, semesterID)
	if err != nil {
		return ustcschool.Semester{}, nil, nil, err
	}
	if len(curricula) == 0 {
		return ustcschool.Semester{}, nil, nil, fmt.Errorf("no school lesson codes found for %s", source.Program)
	}

	var firstSemester ustcschool.Semester
	var firstLifeSemester map[string]any
	sectionsByCourse := map[string]map[string]any{}
	for index, curriculum := range curricula {
		if index == 0 {
			firstSemester = curriculum.Semester
		}
		codes := uniqueLessonCodes(curriculum.Items)
		if len(codes) == 0 {
			continue
		}
		lifeSemesterID, lifeSemester, ok := resolveLifeSemester(lifeSemesterRaw, curriculum.Semester)
		if !ok {
			continue
		}
		if firstLifeSemester == nil {
			firstLifeSemester = lifeSemester
		}
		matchRaw, err := api.ParseResponseRaw(apiClient.MatchSectionCodes(cmd.Context(), newMatchSectionCodesBody(codes, lifeSemesterID)))
		if err != nil {
			return ustcschool.Semester{}, nil, nil, err
		}
		matchMap := cmdutil.AsMap(matchRaw)
		for course, section := range sectionsByCourseName(curriculum.Items, sectionsByCode(matchMap["sections"])) {
			sectionsByCourse[course] = section
		}
	}
	if len(sectionsByCourse) == 0 {
		return ustcschool.Semester{}, nil, nil, fmt.Errorf("could not map any %s school semesters to Life@USTC sections", source.Program)
	}
	return firstSemester, firstLifeSemester, sectionsByCourse, nil
}

func homeworkSyncCurricula(cmd *cobra.Command, source schoolSyncSource, semesterID int) ([]schoolCurriculumResult, error) {
	if semesterID != 0 || source.Program.IsGraduate() {
		semester, curriculum, err := source.Client.FetchCurriculum(cmd.Context(), semesterID)
		if err != nil {
			return nil, err
		}
		if len(uniqueLessonCodes(curriculum)) == 0 {
			return nil, nil
		}
		return []schoolCurriculumResult{{
			Program:  source.Program,
			Semester: semester,
			Items:    curriculum,
		}}, nil
	}

	curricula, _, err := fetchAllCurricula(cmd, source)
	return curricula, err
}

func mergeHomeworkSyncResults(results []homeworkSyncResult, dryRun bool) homeworkSyncResult {
	merged := homeworkSyncResult{
		Program:           "all",
		SchoolHomework:    []ustcschool.HomeworkItem{},
		Created:           []homeworkSyncItemResult{},
		Matched:           []homeworkSyncItemResult{},
		CompletionUpdated: []homeworkSyncItemResult{},
		Skipped:           []homeworkSyncItemResult{},
		Unmatched:         []homeworkSyncItemResult{},
		Results:           results,
		DryRun:            dryRun,
	}
	if len(results) == 1 {
		results[0].Results = nil
		return results[0]
	}
	for _, result := range results {
		merged.SchoolHomework = append(merged.SchoolHomework, result.SchoolHomework...)
		merged.Created = append(merged.Created, result.Created...)
		merged.Matched = append(merged.Matched, result.Matched...)
		merged.CompletionUpdated = append(merged.CompletionUpdated, result.CompletionUpdated...)
		merged.Skipped = append(merged.Skipped, result.Skipped...)
		merged.Unmatched = append(merged.Unmatched, result.Unmatched...)
	}
	return merged
}

func homeworkSyncPrograms(results []homeworkSyncResult) []string {
	programs := make([]string, 0, len(results))
	for _, result := range results {
		if result.Program != "" {
			programs = append(programs, string(result.Program))
		}
	}
	return programs
}

func sectionsByCode(raw any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, section := range cmdutil.RowsFromAny(raw) {
		code := anyString(section["code"])
		if code != "" {
			out[code] = section
		}
	}
	return out
}

func sectionsByCourseName(curriculum []ustcschool.CurriculumItem, byCode map[string]map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, item := range curriculum {
		section := byCode[item.LessonCode]
		if section == nil {
			continue
		}
		courseName := normalizeHomeworkKey(item.CourseName)
		if courseName != "" {
			out[courseName] = section
		}
	}
	return out
}

func fetchLifeHomeworksForSection(cmd *cobra.Command, apiClient *api.TypedClient, sectionID string) ([]map[string]any, error) {
	includeDeleted :=
		openapi.CommunitySectionHomeworkListParamsIncludeDeletedFalse
	parsedSectionID, err := cmdutil.Int64PtrIfSet(sectionID)
	if err != nil {
		return nil, err
	}
	raw, err := api.ParseResponseRaw(
		apiClient.CommunitySectionHomeworkList(
			cmd.Context(),
			&openapi.CommunitySectionHomeworkListParams{
				SectionId:      parsedSectionID,
				IncludeDeleted: &includeDeleted,
			},
		),
	)
	if err != nil {
		return nil, err
	}
	return cmdutil.NewListResult(raw, "homeworks").Rows, nil
}

func createLifeHomework(cmd *cobra.Command, apiClient *api.TypedClient, sectionID string, item ustcschool.HomeworkItem) (map[string]any, error) {
	sectionIdUnion := openapi.HomeworkCreateRequestSchema_0_SectionId{}
	_ = sectionIdUnion.FromHomeworkCreateRequestSchema0SectionId0(sectionID)
	schemaBody := openapi.HomeworkCreateRequestSchema0{
		SectionId: sectionIdUnion,
		Title:     item.Title,
	}
	if start := lifeHomeworkTime(item.StartAt); start != "" {
		startUnion := openapi.HomeworkCreateRequestSchema_0_SubmissionStartAt{}
		_ = startUnion.FromHomeworkCreateRequestSchema0SubmissionStartAt0(start)
		schemaBody.SubmissionStartAt = &startUnion
	}
	if due := lifeHomeworkTime(item.EndAt); due != "" {
		dueUnion := openapi.HomeworkCreateRequestSchema_0_SubmissionDueAt{}
		_ = dueUnion.FromHomeworkCreateRequestSchema0SubmissionDueAt0(due)
		schemaBody.SubmissionDueAt = &dueUnion
	}
	body := openapi.CommunitySectionHomeworkCreateJSONRequestBody{}
	_ = body.FromHomeworkCreateRequestSchema0(schemaBody)
	raw, err := api.ParseResponseRaw(
		apiClient.CommunitySectionHomeworkCreate(cmd.Context(), body),
	)
	if err != nil {
		return nil, err
	}
	return unwrapHomeworkMap(raw), nil
}

func setLifeHomeworkCompletion(cmd *cobra.Command, apiClient *api.TypedClient, homeworkID string, completed bool) error {
	_, err := api.ParseResponseRaw(apiClient.SetHomeworkCompletion(cmd.Context(), homeworkID, openapi.SetHomeworkCompletionJSONRequestBody{
		Completed: completed,
	}))
	return err
}

func matchLifeHomework(existing []map[string]any, item ustcschool.HomeworkItem) map[string]any {
	title := normalizeHomeworkKey(item.Title)
	due := normalizedHomeworkTimeKey(item.EndAt)
	for _, row := range existing {
		if normalizeHomeworkKey(anyString(row["title"])) != title {
			continue
		}
		if normalizedHomeworkTimeKey(anyString(row["submissionDueAt"])) == due {
			return row
		}
	}
	return nil
}

func unwrapHomeworkMap(raw any) map[string]any {
	row := cmdutil.AsMap(raw)
	if row == nil {
		return nil
	}
	if homework := cmdutil.AsMap(row["homework"]); homework != nil {
		return homework
	}
	return row
}

func schoolHomeworkCompleted(item ustcschool.HomeworkItem) bool {
	completed, _ := schoolHomeworkCompletion(item)
	return completed
}

func schoolHomeworkCompletion(item ustcschool.HomeworkItem) (bool, bool) {
	switch normalizeHomeworkKey(item.Status) {
	case "submitted", "graded":
		return true, true
	case "已提交", "已评分", "需要评分":
		return true, true
	case "pending", "overdue":
		return false, true
	case "尚未提交", "尚未评分":
		return false, true
	default:
		return false, false
	}
}

func normalizeHomeworkKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func lifeHomeworkTime(value string) string {
	parsed, ok := parseHomeworkTime(value)
	if !ok {
		return ""
	}
	return parsed.Format(time.RFC3339)
}

func normalizedHomeworkTimeKey(value string) string {
	parsed, ok := parseHomeworkTime(value)
	if !ok {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}

func parseHomeworkTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, schoolTimeLocation); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func homeworkSyncRows(result homeworkSyncResult) []map[string]any {
	var rows []map[string]any
	for _, group := range [][]homeworkSyncItemResult{result.Created, result.Matched, result.Unmatched, result.Skipped} {
		for _, item := range group {
			rows = append(rows, map[string]any{
				"action":     item.Action,
				"course":     item.SchoolHomework.CourseName,
				"title":      item.SchoolHomework.Title,
				"due":        item.SchoolHomework.EndAt,
				"section":    anyString(item.Section["code"]),
				"completion": item.Completion,
				"reason":     item.Reason,
			})
		}
	}
	return rows
}

func extractSectionIDs(raw any) []int {
	sections, ok := raw.([]any)
	if !ok {
		return nil
	}

	ids := make([]int, 0, len(sections))
	for _, section := range sections {
		row, ok := section.(map[string]any)
		if !ok {
			continue
		}
		switch value := row["id"].(type) {
		case float64:
			ids = append(ids, int(value))
		case string:
			id, err := strconv.Atoi(value)
			if err == nil {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func resolveLifeSemester(raw any, schoolSemester ustcschool.Semester) (string, map[string]any, bool) {
	result := cmdutil.NewListResult(raw, "data")
	if len(result.Rows) == 0 {
		return "", nil, false
	}

	schoolCode := strings.TrimSpace(schoolSemester.Code)
	schoolID := strconv.Itoa(schoolSemester.ID)
	schoolName := strings.TrimSpace(schoolSemester.Name())

	var fallbackID string
	var fallbackRow map[string]any
	for _, row := range result.Rows {
		id, ok := anyIntString(row["id"])
		if !ok {
			continue
		}
		if jwID, ok := anyInt(row["jwId"]); ok && jwID == schoolSemester.ID {
			return id, row, true
		}

		code := anyString(row["code"])
		name := anyString(row["nameCn"])
		if code == "" && name == "" {
			continue
		}
		if code == schoolID {
			return id, row, true
		}
		if fallbackID == "" && ((schoolCode != "" && code == schoolCode) || (schoolName != "" && name == schoolName)) {
			fallbackID = id
			fallbackRow = row
		}
	}
	if fallbackID != "" {
		return fallbackID, fallbackRow, true
	}
	return "", nil, false
}

func anyInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func anyIntString(value any) (string, bool) {
	parsed, ok := anyInt(value)
	if !ok {
		return "", false
	}
	return strconv.Itoa(parsed), true
}

func anyString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
