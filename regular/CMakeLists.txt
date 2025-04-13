cmake_minimum_required(VERSION 3.29)
project(monte_carlo)

find_package(xtensor REQUIRED)


set(CMAKE_CXX_STANDARD 14)

add_executable(monte_carlo main.cpp)
target_include_directories(monte_carlo  PUBLIC ${xtensor_INCLUDE_DIRS})
target_link_libraries(monte_carlo PUBLIC xtensor)