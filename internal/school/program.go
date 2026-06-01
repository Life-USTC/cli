package school

type Program string

const (
	ProgramUndergraduate Program = "undergraduate"
	ProgramGraduate      Program = "graduate"
)

func (p Program) IsGraduate() bool {
	return p == ProgramGraduate
}
