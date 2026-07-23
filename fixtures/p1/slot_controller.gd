## Coordinates deterministic slot simulation commands.
class_name SlotController
extends RefCounted

## Emitted after a spin resolves.
signal spin_resolved(result: SpinResult)

@export
var reel_board: ReelBoard
var coins: int = 0

## Creates a controller for the supplied board.
func _init(board: ReelBoard) -> void:
	reel_board = board

## Resolves one deterministic spin.
func spin(
	seed: int,
	context: SpinContext = null,
) -> SpinResult:
	return reel_board.spin(seed, context)

## Creates a default controller.
static func create(board: ReelBoard) -> SlotController:
	return SlotController.new(board)

func _reset_for_test() -> void:
	coins = 0
