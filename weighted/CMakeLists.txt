cmake_minimum_required(VERSION 3.29)
project(weighted_monte_carlo)
find_package(xtensor REQUIRED)
set(CMAKE_CXX_STANDARD 20)

add_executable(weighted_monte_carlo main.cpp)
target_include_directories(weighted_monte_carlo  PUBLIC ${xtensor_INCLUDE_DIRS})
target_link_libraries(weighted_monte_carlo PUBLIC xtensor)

