package engine

import "math"

type environmentSnapshot struct {
	temp TempBand
	loc  LocationType
}

func advanceConditions(s *Survivor, diff Difficulty, choice Choice, lastDelta Stats) conditionOutcome {
	out := conditionOutcome{}
	if s == nil {
		return out
	}
	if s.Meters == nil {
		s.Meters = make(map[Meter]int)
	}
	updateThirstMeters(s, lastDelta)
	updateTemperatureMeters(s)
	updateFeverMeters(s, choice)
	applyDehydrationTriggers(s, &out)
	applyHypothermiaTriggers(s, &out)
	applyExhaustionTriggers(s, &out)
	applyFeverRemoval(s, &out)
	applyConditionRemovals(s, &out)
	out.Delta = addStats(out.Delta, conditionTick(s, diff))
	return out
}

func updateThirstMeters(s *Survivor, lastDelta Stats) {
	thirstStreak := s.Meters[MeterThirstStreak]
	if s.Stats.Thirst >= 80 {
		if thirstStreak < math.MaxInt32 {
			thirstStreak++
		}
	} else {
		thirstStreak = 0
	}
	s.Meters[MeterThirstStreak] = thirstStreak
	recovery := s.Meters[MeterHydrationRecovery]
	if lastDelta.Thirst <= -10 {
		if recovery < 2 {
			recovery++
		}
	} else if recovery > 0 {
		recovery--
	}
	s.Meters[MeterHydrationRecovery] = recovery
}

func updateTemperatureMeters(s *Survivor) {
	exposure := s.Meters[MeterColdExposure]
	if isHypothermiaExposure(s.Environment) {
		if exposure < math.MaxInt32 {
			exposure++
		}
	} else if exposure > 0 {
		exposure--
	}
	s.Meters[MeterColdExposure] = exposure
	warm := s.Meters[MeterWarmStreak]
	if isWarmShelter(s.Environment) {
		if warm < math.MaxInt32 {
			warm++
		}
	} else {
		warm = 0
	}
	s.Meters[MeterWarmStreak] = warm
}

func updateFeverMeters(s *Survivor, choice Choice) {
	rest := s.Meters[MeterFeverRest]
	if rest > 0 {
		rest--
	}
	if choice.Archetype == "rest" {
		if rest < 8 {
			rest++
		}
	}
	s.Meters[MeterFeverRest] = rest
	med := s.Meters[MeterFeverMedication]
	if med > 0 {
		med--
	}
	s.Meters[MeterFeverMedication] = med
}

func applyDehydrationTriggers(s *Survivor, out *conditionOutcome) {
	if s.Meters[MeterThirstStreak] >= 3 {
		if addConditionIfAbsent(s, ConditionDehydration) {
			out.Added = append(out.Added, ConditionDehydration)
		}
	}
}

func applyHypothermiaTriggers(s *Survivor, out *conditionOutcome) {
	if s.Meters[MeterColdExposure] >= 3 {
		if addConditionIfAbsent(s, ConditionHypothermia) {
			out.Added = append(out.Added, ConditionHypothermia)
		}
	}
}

func applyExhaustionTriggers(s *Survivor, out *conditionOutcome) {
	if s.Stats.Fatigue >= 85 {
		if addConditionIfAbsent(s, ConditionExhaustion) {
			out.Added = append(out.Added, ConditionExhaustion)
		}
	}
}

func applyFeverRemoval(s *Survivor, out *conditionOutcome) {
	if !survivorHasCondition(*s, ConditionFever) {
		return
	}
	if s.Meters[MeterFeverMedication] > 0 && s.Meters[MeterFeverRest] >= 6 {
		if removeConditionIfPresent(s, ConditionFever) {
			out.Removed = append(out.Removed, ConditionFever)
			s.Meters[MeterFeverRest] = 0
			s.Meters[MeterFeverMedication] = 0
		}
	}
}

func applyConditionRemovals(s *Survivor, out *conditionOutcome) {
	if survivorHasCondition(*s, ConditionDehydration) {
		if s.Stats.Thirst <= 40 && s.Meters[MeterHydrationRecovery] >= 2 {
			if removeConditionIfPresent(s, ConditionDehydration) {
				out.Removed = append(out.Removed, ConditionDehydration)
				s.Meters[MeterHydrationRecovery] = 0
			}
		}
	}
	if survivorHasCondition(*s, ConditionHypothermia) {
		if s.Meters[MeterWarmStreak] >= 4 {
			if removeConditionIfPresent(s, ConditionHypothermia) {
				out.Removed = append(out.Removed, ConditionHypothermia)
				s.Meters[MeterWarmStreak] = 0
				s.Meters[MeterColdExposure] = 0
			}
		}
	}
	if survivorHasCondition(*s, ConditionExhaustion) {
		if s.Stats.Fatigue <= 50 {
			if removeConditionIfPresent(s, ConditionExhaustion) {
				out.Removed = append(out.Removed, ConditionExhaustion)
				s.Meters[MeterExhaustionScenes] = 0
			}
		}
	}
}

func conditionTick(s *Survivor, diff Difficulty) Stats {
	total := Stats{}
	for _, cond := range s.Conditions {
		switch cond {
		case ConditionBleeding:
			total.Health -= bleedDamage(diff)
			total.Fatigue += 2
		case ConditionDehydration:
			total.Fatigue += 2
			total.Morale -= 2
			total.Health -= dehydrationDamage(diff)
		case ConditionFever:
			total.Fatigue += 1
			total.Morale -= 2
			total.Health -= feverDamage(diff)
		case ConditionHypothermia:
			total.Fatigue += 2
			total.Health -= hypothermiaDamage(diff)
		case ConditionExhaustion:
			s.Meters[MeterExhaustionScenes]++
			if s.Meters[MeterExhaustionScenes] >= 4 {
				total.Health -= exhaustionDamage(diff)
			}
		}
	}
	if !survivorHasCondition(*s, ConditionExhaustion) {
		s.Meters[MeterExhaustionScenes] = 0
	}
	return total
}

func bleedDamage(diff Difficulty) int {
	switch diff {
	case DifficultyEasy:
		return 4
	case DifficultyHard:
		return 8
	default:
		return 6
	}
}

func dehydrationDamage(diff Difficulty) int {
	switch diff {
	case DifficultyEasy:
		return 0
	default:
		return 1
	}
}

func feverDamage(diff Difficulty) int {
	switch diff {
	case DifficultyEasy:
		return 0
	default:
		return 1
	}
}

func hypothermiaDamage(diff Difficulty) int {
	switch diff {
	case DifficultyEasy:
		return 2
	case DifficultyHard:
		return 4
	default:
		return 3
	}
}

func exhaustionDamage(diff Difficulty) int {
	switch diff {
	case DifficultyHard:
		return 2
	default:
		return 1
	}
}

func isHypothermiaExposure(env Environment) bool {
	cold := env.TempBand == TempFreezing || env.TempBand == TempCold
	if !cold {
		return false
	}
	switch env.Location {
	case LocationForest, LocationCoast, LocationMountain, LocationRural:
		return true
	default:
		return false
	}
}

func isWarmShelter(env Environment) bool {
	return env.TempBand == TempMild || env.TempBand == TempWarm || env.TempBand == TempHot
}
